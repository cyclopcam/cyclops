package eventdb

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"

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
	fullPath := filepath.Join(e.root, recording.Filename())
	os.MkdirAll(filepath.Dir(fullPath), 0777)
	e.log.Infof("Saving recording %v", fullPath)
	if err := buf.SaveToMP4(fullPath); err != nil {
		return err
	}
	return e.db.Create(recording).Error
}
