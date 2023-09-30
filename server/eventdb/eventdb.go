package eventdb

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/bmharper/cimg/v2"
	"github.com/cyclopcam/cyclops/pkg/dbh"
	"github.com/cyclopcam/cyclops/pkg/log"
	"github.com/cyclopcam/cyclops/server/defs"
	"github.com/cyclopcam/cyclops/server/videox"
	"gorm.io/gorm"
)

const MaxThumbnailWidth = 320

var (
	ErrNotALogicalRecord = errors.New("Not a logical record")
)

// EventDB manages recordings.
// There are two EventDBs.
// One for recent recordings, which may or may not be of interest.
// One for permanent recordings, which form part of the training set (or a user wants to keep for whatever reason).
type EventDB struct {
	Root string // Where we store our videos (also directory where sqlite DB is stored)

	log log.Log
	db  *gorm.DB
}

// Open or create an event DB
func Open(log log.Log, root string) (*EventDB, error) {
	root = filepath.Clean(root)
	if err := os.MkdirAll(root, 0777); err != nil {
		return nil, fmt.Errorf("Failed to set event storage path '%v': %w", root, err)
	}

	log.Infof("Opening Events DB at '%v'", root)
	dbPath := filepath.Join(root, "events.sqlite")
	eventDB, err := dbh.OpenDB(log, dbh.MakeSqliteConfig(dbPath), Migrations(log), 0)
	if err == nil {
		self := &EventDB{
			log:  log,
			db:   eventDB,
			Root: root,
		}
		if err := LoadStandardOntology(self); err != nil {
			return nil, err
		}
		return self, nil
	} else {
		err = fmt.Errorf("Failed to open database %v: %w", dbPath, err)
	}
	return nil, err
}

// Save a new recording to disk.
// Returns the record of the new recording.
func (e *EventDB) Save(res defs.Resolution, origin RecordingOrigin, cameraID int64, startTime time.Time, buf *videox.RawBuffer) (*Recording, error) {
	rnd, err := e.createRandomID()
	if err != nil {
		return nil, err
	}
	width, height, err := buf.DecodeHeader()
	if err != nil {
		return nil, err
	}
	recording := &Recording{
		RandomID:   rnd,
		StartTime:  dbh.MakeIntTime(startTime),
		RecordType: RecordTypeSimple,
		Origin:     origin,
		CameraID:   cameraID,
	}
	recording.SetFormatAndDimensions(res, width, height)
	videoPath := e.FullPath(recording.VideoFilename(res))
	thumbnailPath := e.FullPath(recording.ThumbnailFilename())
	os.MkdirAll(filepath.Dir(videoPath), 0770)
	e.log.Infof("Creating recording thumbnail %v", thumbnailPath)
	if err := e.saveThumbnailFromVideo(buf, thumbnailPath); err != nil {
		return nil, err
	}
	e.log.Infof("Saving recording %v", videoPath)
	if err := buf.SaveToMP4(videoPath); err != nil {
		return nil, err
	}

	videoStat, err := os.Stat(videoPath)
	if err != nil {
		// soft fail
		e.log.Errorf("Failed to stat newly created video %v: %v", videoPath, err)
		recording.Bytes += 1024 * 1024
	} else {
		recording.Bytes += videoStat.Size()
	}

	thumbStat, err := os.Stat(thumbnailPath)
	if err != nil {
		// soft fail
		e.log.Errorf("Failed to stat newly created thumbnail %v: %v", thumbnailPath, err)
	} else {
		recording.Bytes += thumbStat.Size()
	}

	if err := e.db.Create(recording).Error; err != nil {
		return nil, err
	}
	return recording, nil
}

// Create a new empty recording
// The idea is that you'll be building this recording's mp4 file bit by bit.
func (e *EventDB) CreateRecording(parentID int64, rtype RecordType, origin RecordingOrigin, startTime time.Time, cameraID int64, res defs.Resolution, width, height int) (*Recording, error) {
	rnd, err := e.createRandomID()
	if err != nil {
		return nil, err
	}
	recording := &Recording{
		RandomID:   rnd,
		StartTime:  dbh.MakeIntTime(startTime),
		RecordType: rtype,
		Origin:     origin,
		ParentID:   parentID,
		CameraID:   cameraID,
	}
	recording.SetFormatAndDimensions(res, width, height)
	if err := e.db.Create(recording).Error; err != nil {
		return nil, err
	}
	return recording, nil
}

// Delete the _DB record only_ of the recording
// If the record does not exist, return nil.
func (e *EventDB) DeleteRecordingDBRecord(id int64) error {
	return e.db.Where("id = ? OR parent_id = ?", id, id).Delete(&Recording{}, id).Error
}

// Delete the DB record and the video files of a recording.
// If the recording does not exist, the function returns success.
func (e *EventDB) DeleteRecordingComplete(id int64) error {
	rec := Recording{}
	err := e.db.First(&rec, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	} else if err != nil {
		return err
	}

	// Get all physical records before deleting from the DB
	physical, err := e.GetPhysicalRecordsOf(&rec)
	if err != nil {
		return err
	}

	e.log.Infof("Deleting recording %v", id)

	// Delete the DB record first, and then the video files.
	// It would be worse to have a DB record sticking around, with missing files.
	if err := e.DeleteRecordingDBRecord(id); err != nil {
		return err
	}

	return e.DeleteFilesOf(physical)
}

// Get all physical records for the given logical or simple recording object
func (e *EventDB) GetPhysicalRecordsOf(rec *Recording) ([]*Recording, error) {
	if rec.IsSimple() || rec.IsPhysical() {
		return []*Recording{rec}, nil
	}
	physical := []*Recording{}
	if err := e.db.Where("parent_id = ?", rec.ID).Find(&physical).Error; err != nil {
		return nil, err
	}
	return physical, nil
}

// Delete the video files (but not the DB record) of the given recordings
// The recording records should be Simple or Physical records
func (e *EventDB) DeleteFilesOf(recordings []*Recording) error {
	// keep on trucking if we fail to delete a file
	var firstErr error
	for _, rec := range recordings {
		all := []string{
			e.FullPath(rec.VideoFilenameHD()),
			e.FullPath(rec.VideoFilenameLD()),
			e.FullPath(rec.ThumbnailFilename()),
		}
		for _, fn := range all {
			if err := deleteIfExists(fn); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

func (e *EventDB) GetRecording(id int64) (*Recording, error) {
	rec := Recording{}
	if err := e.db.First(&rec, id).Error; err != nil {
		return nil, err
	}
	return &rec, nil
}

func (e *EventDB) GetRecordings() ([]Recording, error) {
	recordings := []Recording{}
	if err := e.db.Where("record_type IN (?,?)", RecordTypeLogical, RecordTypeSimple).Find(&recordings).Error; err != nil {
		return nil, err
	}
	return recordings, nil
}

func (e *EventDB) GetRecordingsForTraining() ([]Recording, error) {
	recordings := []Recording{}
	if err := e.db.Where("record_type IN (?,?) AND use_for_training = 1", RecordTypeLogical, RecordTypeSimple).Find(&recordings).Error; err != nil {
		return nil, err
	}
	return recordings, nil
}

func (e *EventDB) Count() (int64, error) {
	n := int64(0)
	if err := e.db.Model(&Recording{}).Where("record_type IN (?,?)", RecordTypeLogical, RecordTypeSimple).Count(&n).Error; err != nil {
		return 0, err
	}
	return n, nil
}

func (e *EventDB) SetRecordingLabels(rec *Recording) error {
	return e.db.Model(&rec).Select("use_for_training", "labels", "ontology_id").Updates(rec).Error
}

func (e *EventDB) GetOntology(id int64) (*Ontology, error) {
	ontology := Ontology{}
	if err := e.db.First(&ontology, id).Error; err != nil {
		return nil, err
	}
	return &ontology, nil
}

func (e *EventDB) GetOntologies() ([]Ontology, error) {
	ontologies := []Ontology{}
	if err := e.db.Find(&ontologies).Error; err != nil {
		return nil, err
	}
	return ontologies, nil
}

func (e *EventDB) GetLatestOntology() (*Ontology, error) {
	ontology := Ontology{}
	if err := e.db.Order("created_at DESC").First(&ontology).Error; err != nil {
		return nil, err
	}
	return &ontology, nil
}

// Return true if there are any recordings that reference the given ontology
func (e *EventDB) IsOntologyUsed(id int64) (bool, error) {
	n := int64(0)
	if err := e.db.Model(&Recording{}).Where("ontology_id = ?", id).Count(&n).Error; err != nil {
		return false, err
	}
	return n != 0, nil
}

// Find an existing ontology that matches the given spec, or create a new one if necessary
func (e *EventDB) CreateOntology(spec *OntologyDefinition) (int64, error) {
	existing, err := e.GetOntologies()
	if err != nil {
		return 0, err
	}
	// Look for existing
	specHash := spec.Hash()
	for i := range existing {
		if string(existing[i].Definition.Data.Hash()) == string(specHash) {
			return existing[i].ID, nil
		}
	}
	// Create new
	now := time.Now()
	ontology := &Ontology{
		CreatedAt:  dbh.MakeIntTime(now),
		Definition: dbh.MakeJSONField(*spec),
	}
	if err := e.db.Create(ontology).Error; err != nil {
		return 0, err
	}
	return ontology.ID, nil
}

// Find the latest ontology that is a superset of the given spec.
// Returns (nil, nil) if no such ontology exists.
func (e *EventDB) FindLatestOntologyThatIsSupersetOf(spec *OntologyDefinition) (*Ontology, error) {
	// Try first for the most likely scenario. This avoids a gradual slowdown over time,
	// as more historical ontologies fill up the DB.
	latest, err := e.GetLatestOntology()
	if latest != nil && latest.Definition.Data.IsSupersetOf(spec) {
		return latest, nil
	}
	latest = nil

	all, err := e.GetOntologies()
	if err != nil {
		return nil, err
	}
	for i := range all {
		if all[i].Definition.Data.IsSupersetOf(spec) {
			if latest == nil || all[i].CreatedAt.Get().After(latest.CreatedAt.Get()) {
				latest = &all[i]
			}
		}
	}
	return nil, nil
}

// Delete unused ontologies.
// The optional array 'keep' will prevent ontologies with those IDs from being deleted.
func (e *EventDB) PruneUnusedOntologies(keep []int64) error {
	db, err := e.db.DB()
	if err != nil {
		return err
	}
	if len(keep) == 0 {
		_, err = db.Exec("DELETE FROM ontology WHERE id NOT IN (SELECT distinct(ontology_id) FROM recording WHERE ontology_id IS NOT NULL)")
	} else {
		_, err = db.Exec("DELETE FROM ontology WHERE id NOT IN (SELECT distinct(ontology_id) FROM recording WHERE ontology_id IS NOT NULL) AND id NOT IN " + dbh.IDListToSQLSet(keep))
	}
	return err
}

// Return the complete path to the specified video or image file
// For example, eventDB.FullPath(recording.VideoFilenameLD())
func (e *EventDB) FullPath(videoOrImagePath string) string {
	return filepath.Join(e.Root, videoOrImagePath)
}

func (e *EventDB) createRandomID() (string, error) {
	rnd := [4]byte{}
	if _, err := rand.Read(rnd[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(rnd[:]), nil
}

func (e *EventDB) saveThumbnailFromVideo(buf *videox.RawBuffer, targetFilename string) error {
	img, err := buf.ExtractThumbnail()
	if err != nil {
		// If thumbnail creation fails, it's a good sign that this video is useless
		return fmt.Errorf("Failed to decode video while creating thumbnail: %w", err)
	}
	return e.SaveThumbnail(img, targetFilename)
}

func (e *EventDB) SaveThumbnail(img *cimg.Image, targetFilename string) error {
	if img.Width > MaxThumbnailWidth {
		img = cimg.ResizeNew(img, MaxThumbnailWidth, (MaxThumbnailWidth*img.Height)/img.Width)
	}
	b, err := cimg.Compress(img, cimg.MakeCompressParams(cimg.Sampling420, 80, 0))
	if err != nil {
		return err
	}
	return os.WriteFile(targetFilename, b, 0660)
}

func deleteIfExists(filename string) error {
	err := os.Remove(filename)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
