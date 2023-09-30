package train

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/cyclopcam/cyclops/pkg/log"
	"github.com/cyclopcam/cyclops/server/eventdb"
)

// Trainer manages the training of the model.
type Trainer struct {
	Log    log.Log
	permDB *eventdb.EventDB
}

func NewTrainer(log log.Log, permDB *eventdb.EventDB) *Trainer {
	return &Trainer{
		Log:    log,
		permDB: permDB,
	}
}

// Extracts the training data as a single archive file.
func (t *Trainer) GetDataset(w io.Writer) error {
	recordings, err := t.permDB.GetRecordingsForTraining()
	if err != nil {
		return err
	}

	zipWriter := zip.NewWriter(w)
	defer zipWriter.Close()

	// Flatten all ontologies into one,

	// Sort recordings by ontology ID, so that we see later ontologies after earlier ones.
	// Later ontologies override earlier ones.
	// For example, if an early ontology says that the tag named "car" is alert level "record",
	// but a later ontology says that "car" is alert level "alarm", then we pick the later one.
	sort.SliceStable(recordings, func(i, j int) bool {
		return recordings[i].OntologyID < recordings[j].OntologyID
	})

	mergedOntology := &eventdb.OntologyDefinition{}
	seenOntology := map[int64]bool{}
	tagNameToIndex := map[string]int{} // Map from "ontologyID:tagIndex" to index in mergedOntology.Tags

	for _, recording := range recordings {
		if !seenOntology[recording.OntologyID] {
			seenOntology[recording.OntologyID] = true
			ontology, err := t.permDB.GetOntology(recording.OntologyID)
			if err != nil {
				return err
			}
			for iTag, tag := range ontology.Definition.Data.Tags {
				found := false
				for iExisting := range mergedOntology.Tags {
					if strings.EqualFold(mergedOntology.Tags[iExisting].Name, tag.Name) {
						// As described in the comment block above, later ontologies override earlier ones.
						mergedOntology.Tags[iExisting].Level = tag.Level
						found = true
					}
				}
				if !found {
					tagNameToIndex[fmt.Sprintf("%v:%v", ontology.ID, iTag)] = len(mergedOntology.Tags)
					mergedOntology.Tags = append(mergedOntology.Tags, tag)
				}
			}
		}
	}

	// Rewrite tag indices in all recordings, so that they all refer to the merged ontology.
	for i := range recordings {
		for iTag, tag := range recordings[i].Labels.Data.VideoTags {
			mergedTagIndex, ok := tagNameToIndex[fmt.Sprintf("%v:%v", recordings[i].OntologyID, tag)]
			if !ok {
				return fmt.Errorf("Video tag %v in recording %v is an invalid index for Ontology %v", tag, recordings[i].ID, recordings[i].OntologyID)
			}
			recordings[i].Labels.Data.VideoTags[iTag] = mergedTagIndex
		}
	}

	for _, recording := range recordings {
		videoZ, err := zipWriter.Create(fmt.Sprintf("cameras/%v/videos/%v.mp4", recording.CameraID, recording.ID))
		if err != nil {
			return err
		}
		src := t.permDB.FullPath(recording.VideoFilenameLD())
		if err := copyFile(videoZ, src); err != nil {
			return err
		}

		labelsZ, err := zipWriter.Create(fmt.Sprintf("cameras/%v/labels/%v.json", recording.CameraID, recording.ID))
		if err != nil {
			return err
		}
		if err := json.NewEncoder(labelsZ).Encode(recording.Labels.Data); err != nil {
			return err
		}
	}

	ontologyZ, err := zipWriter.Create("ontology.json")
	if err != nil {
		return err
	}
	if err := json.NewEncoder(ontologyZ).Encode(mergedOntology); err != nil {
		return err
	}

	return nil
}

func copyFile(dst io.Writer, src string) error {
	file, err := os.Open(src)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(dst, file)
	return err
}
