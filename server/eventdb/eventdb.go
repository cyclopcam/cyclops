package eventdb

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
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
	os.Remove(dbPath)
	eventDB, err := dbh.OpenDB(log, dbh.DriverSqlite, dbPath, Migrations(log), 0)
	/*
		xx := dbh.DBConfig{
			Driver:   "postgres",
			Host:     "localhost",
			Port:     5432,
			Database: "foo",
			Username: "postgres",
			Password: "password",
		}
		dbPath = xx.DSN()
		eventDB, err := dbh.OpenDB(log, dbh.DriverPostgres, xx.DSN(), Migrations(log), dbh.DBConnectFlagWipeDB)
	*/
	if err == nil {
		log.Infof("create1")
		labels := Labels{
			Tags: []string{"foox", "barz"},
		}
		//jj, _ := json.Marshal(Labels{
		//	Tags: []string{"foo", "bar"},
		//})
		rec := &Recording{
			RandomID:  "123",
			StartTime: dbh.MakeIntTime(time.Now()),
			//FooTime:   dbh.Milli(time.Now()),
			BarTime: dbh.MakeIntTime(time.Now()),
			Format:  "mp4",
			//Labels:    &jf,
			Labels: MakeJSONField(labels),
			//Labels2: MakeJSONField2(labels),
		}
		ee := eventDB.Create(rec).Error
		log.Infof("create2: %v", ee)

		rec = &Recording{
			RandomID:  "555",
			StartTime: dbh.MakeIntTime(time.Now()),
			Format:    "mp4",
		}
		ee = eventDB.Create(rec).Error
		log.Infof("create3: %v", ee)

		rec2 := Recording{}
		ee = eventDB.First(&rec2, "random_id = '123'").Error
		log.Infof("fetch1: %v", ee)
		log.Infof("fetch1: %v", rec2)
		rec2J, _ := json.Marshal(&rec2)
		log.Infof("fetch1J: %v", string(rec2J))

		rec3 := Recording{}
		ee = eventDB.First(&rec3, "random_id = '555'").Error
		log.Infof("fetch2: %v", ee)
		log.Infof("fetch2: %v", rec3)
		//log.Infof("fetch2 isFooTimeZero: %v", rec3.FooTime.IsZero())
		rec3J, _ := json.Marshal(&rec3)
		log.Infof("fetch2J: %v", string(rec3J))

		zeroTime := time.Time{}
		log.Infof("zeroTime: %v", zeroTime)

		return nil, fmt.Errorf("foo!")

		//return &EventDB{
		//	log:  log,
		//	db:   eventDB,
		//	root: root,
		//}, nil
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
