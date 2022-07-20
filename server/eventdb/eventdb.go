package eventdb

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/bmharper/cimg/v2"
	"github.com/bmharper/cyclops/server/dbh"
	"github.com/bmharper/cyclops/server/log"
	"github.com/bmharper/cyclops/server/videox"
	"gorm.io/gorm"
)

// EventDB manages recordings.
// There are two EventDBs.
// One for recent recordings, which may or may not be of interest.
// One for permanent recordings, which form part of the training set (or a user wants to keep for whatever reason).
type EventDB struct {
	log  log.Log
	db   *gorm.DB
	root string // Where we store our videos (also directory where sqlite DB is stored)
}

// Open or create an event DB
func Open(log log.Log, root string) (*EventDB, error) {
	root = filepath.Clean(root)
	if err := os.MkdirAll(root, 0777); err != nil {
		return nil, fmt.Errorf("Failed to set event storage path '%v': %w", root, err)
	}

	log.Infof("Opening DB at '%v'", root)
	dbPath := filepath.Join(root, "events.sqlite")
	eventDB, err := dbh.OpenDB(log, dbh.DriverSqlite, dbPath, Migrations(log), 0)
	if err == nil {
		return &EventDB{
			log:  log,
			db:   eventDB,
			root: root,
		}, nil
	} else {
		err = fmt.Errorf("Failed to open database %v: %w", dbPath, err)
	}
	return nil, err
}

// Save a new recording to disk
func (e *EventDB) Save(buf *videox.RawBuffer) error {
	rnd := [4]byte{}
	if _, err := rand.Read(rnd[:]); err != nil {
		return err
	}
	recording := &Recording{
		RandomID:  hex.EncodeToString(rnd[:]),
		StartTime: dbh.MakeIntTime(time.Now()),
		Format:    "mp4",
	}
	videoPath := filepath.Join(e.root, recording.VideoFilename())
	thumbnailPath := filepath.Join(e.root, recording.ThumbnailFilename())
	os.MkdirAll(filepath.Dir(videoPath), 0770)
	e.log.Infof("Saving recording %v", videoPath)
	if err := e.saveThumbnail(buf, thumbnailPath); err != nil {
		return err
	}
	if err := buf.SaveToMP4(videoPath); err != nil {
		return err
	}
	return e.db.Create(recording).Error
}

func (e *EventDB) saveThumbnail(buf *videox.RawBuffer, targetFilename string) error {
	img, err := buf.ExtractThumbnail()
	if err != nil {
		// If thumbnail creation fails, it's a good sign that this video is useless
		return fmt.Errorf("Failed to decode video while creating thumbnail: %w", err)
	}
	im, err := cimg.FromImage(img, false)
	if err != nil {
		return err
	}
	b, err := cimg.Compress(im, cimg.MakeCompressParams(cimg.Sampling420, 80, 0))
	if err != nil {
		return err
	}
	return os.WriteFile(targetFilename, b, 0660)
}
