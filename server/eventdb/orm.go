package eventdb

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"

	"github.com/cyclopcam/cyclops/pkg/dbh"
	"github.com/cyclopcam/cyclops/pkg/gen"
	"github.com/cyclopcam/cyclops/server/defs"
)

// BaseModel is our base class for a GORM model.
// The default GORM Model uses int, but we prefer int64
type BaseModel struct {
	ID int64 `gorm:"primaryKey" json:"id"`
}

// RecordType categorizes the type of Recording record in the database.
// The reason we have 3 different types, is because we want to be able to
// group together a bunch of different video files into one logical entity.
// There are two reasons for this:
//  1. When recording for a long time, the video files can become large,
//     and we want to be able to split them up into smaller files on disk.
//  2. We will often record on all cameras at the same time, and we want
//     them to be grouped together into one logical entity.
type RecordType string

const (
	// A recording that is both logical and physical.
	// Has a camera.
	// Parent is null.
	RecordTypeSimple RecordType = "s"

	// A logical record that is the parent of Physical records.
	// Doesn't have a camera.
	// Doesn't have any directly owned files (but it does have files *indirectly*, through it's Physical children).
	RecordTypeLogical RecordType = "l"

	// A recording file, which belongs to a Logical recording.
	// Has a camera
	// Has a parent
	RecordTypePhysical RecordType = "p"
)

type RecordingOrigin string

const (
	RecordingOriginBackground RecordingOrigin = "b" // A recording that was made in the background, to collect sample data
	RecordingOriginAuto       RecordingOrigin = "a" // A recording that was made automatically by the system, because it thought it was interesting or suspicious
	RecordingOriginExplicit   RecordingOrigin = "e" // A recording that was made by the user clicking the "record" button (for labelling)
)

// A recording refers to either a high resolution video, a low resolution video, or both.
// When recording explicitly for training, we record just a low resolution video, because
// that's all we need.
// However, when recording a suspicious event, we want high resolution for playback and inspection,
// but we also want low resolution in case the user wants to turn that video clip into
// training data. This is why we record both high and low whenever the auto recorder kicks in.
type Recording struct {
	BaseModel
	RandomID       string                 `json:"randomID"`                                 // Used to ensure uniqueness when merging event databases
	StartTime      dbh.IntTime            `json:"startTime"`                                // Wall time when recording started
	RecordType     RecordType             `json:"recordType"`                               // Type of record
	Origin         RecordingOrigin        `json:"origin"`                                   // Reason why recording exists
	ParentID       int64                  `json:"parentID" gorm:"default:null"`             // ID of parent recording record, if this is a Physical record
	FormatHD       string                 `json:"formatHD" gorm:"default:null"`             // Only valid value is "mp4"
	FormatLD       string                 `json:"formatLD" gorm:"default:null"`             // Only valid value is "mp4"
	Labels         *dbh.JSONField[Labels] `json:"labels,omitempty" gorm:"default:null"`     // If labels is defined, then OntologyID is also defined
	UseForTraining bool                   `json:"useForTraining" gorm:"default:null"`       // If 1, then this recording will be used for training
	OntologyID     int64                  `json:"ontologyID,omitempty" gorm:"default:null"` // Labels reference indices in Ontology, which is why we need to store a reference to the Ontology
	Bytes          int64                  `json:"bytes"`                                    // Total storage of videos + thumbnails
	DimensionsHD   string                 `json:"dimensionsHD" gorm:"default:null"`         // "Width,Height" of HD video
	DimensionsLD   string                 `json:"dimensionsLD" gorm:"default:null"`         // "Width,Height" of LD video
	CameraID       int64                  `json:"cameraID" gorm:"default:null"`             // ID of camera in config DB
}

func (r *Recording) IsLogical() bool {
	return r.RecordType == RecordTypeLogical
}

func (r *Recording) IsPhysical() bool {
	return r.RecordType == RecordTypePhysical
}

func (r *Recording) IsSimple() bool {
	return r.RecordType == RecordTypeSimple
}

func (r *Recording) VideoFilename(res defs.Resolution) string {
	switch res {
	case defs.ResHD:
		return r.VideoFilenameHD()
	case defs.ResLD:
		return r.VideoFilenameLD()
	}
	panic("Invalid resolution '" + res + "'")
}

func (r *Recording) VideoFilenameHD() string {
	return r.baseFilename() + "-HD." + r.FormatHD
}

func (r *Recording) VideoFilenameLD() string {
	return r.baseFilename() + "-LD." + r.FormatLD
}

func (r *Recording) ThumbnailFilename() string {
	return r.baseFilename() + ".jpg"
}

func (r *Recording) SetFormatAndDimensions(res defs.Resolution, width, height int) {
	if res == defs.ResHD {
		r.FormatHD = "mp4"
		r.DimensionsHD = fmt.Sprintf("%v,%v", width, height)
	} else if res == defs.ResLD {
		r.FormatLD = "mp4"
		r.DimensionsLD = fmt.Sprintf("%v,%v", width, height)
	} else {
		panic("Unknown res")
	}
}

func videoContentType(format string) string {
	switch format {
	case "mp4":
		return "video/mp4"
	}
	panic("Unrecognized video type " + format)
}

func (r *Recording) VideoContentType(res defs.Resolution) string {
	switch res {
	case defs.ResHD:
		return r.VideoContentTypeHD()
	case defs.ResLD:
		return r.VideoContentTypeLD()
	}
	panic("Invalid resolution '" + res + "'")
}

func (r *Recording) VideoContentTypeHD() string {
	return videoContentType(r.FormatHD)
}

func (r *Recording) VideoContentTypeLD() string {
	return videoContentType(r.FormatLD)
}

// Our filename system is based on time and a random seed, to make it easy to copy videos on the filesystem,
// and in particular, to merge them. I'm not sure if this will ever be used.
// Another benefit of this naming convention is that it makes the footage easy to scan through by just
// looking at the file system. Perhaps for forensics, if a device is damaged or something.
func (r *Recording) baseFilename() string {
	t := r.StartTime.Get().UTC()
	return t.Format("2006-01/02/15-04-05-") + fmt.Sprintf("%04d-", r.CameraID) + r.RandomID
}

// Labels associated with a recording
type Labels struct {
	VideoTags []int   `json:"videoTags"` // Tags associated with the entire recording (eg "intruder"). Values refer to zero-based indices of OntologyDefinition.VideoTags
	CropStart float64 `json:"cropStart"` // Start time of cropped video, in seconds
	CropEnd   float64 `json:"cropEnd"`   // End time of cropped video, in seconds
}

// An immutable ontology, referenced by a Recording, so that we know all the possible
// labels which were considered when a recording was labeled.
type Ontology struct {
	BaseModel
	CreatedAt  dbh.IntTime                        `json:"createdAt" gorm:"autoCreateTime:milli"`
	Definition *dbh.JSONField[OntologyDefinition] `json:"definition,omitempty"`
}

// Ontology spec, which is saved as a JSON field in the DB
type OntologyDefinition struct {
	Tags []OntologyTag `json:"tags"`
}

type OntologyLevel string

const (
	// SYNC-ONTOLOGY-LEVEL
	OntologyLevelAlarm  OntologyLevel = "alarm"  // If the system is armed, trigger an alarm
	OntologyLevelRecord OntologyLevel = "record" // Record this incident, whether armed or not
	OntologyLevelIgnore OntologyLevel = "ignore" // Do not record
)

// Ontology tag, which can be associated with a video clip
type OntologyTag struct {
	Name  string        `json:"name"` // eg "intruder", "dog", "car"
	Level OntologyLevel `json:"level"`
}

// Compute a hash that can be used to check for equality with another ontology
func (o *OntologyDefinition) Hash() []byte {
	tags := gen.CopySlice(o.Tags)
	sort.Slice(tags, func(i, j int) bool {
		return tags[i].Name < tags[j].Name
	})
	hs := strings.Builder{}
	for i := range tags {
		hs.WriteString(tags[i].Name)
		hs.WriteString(string(tags[i].Level))
	}
	hash := sha256.Sum256([]byte(hs.String()))
	return hash[:]
}

func (o *OntologyDefinition) IsSupersetOf(b *OntologyDefinition) bool {
	for i := range b.Tags {
		if !o.ContainsTag(b.Tags[i]) {
			return false
		}
	}
	return true
}

func (o *OntologyDefinition) ContainsTag(tag OntologyTag) bool {
	for i := range o.Tags {
		if o.Tags[i].Name == tag.Name && o.Tags[i].Level == tag.Level {
			return true
		}
	}
	return false
}
