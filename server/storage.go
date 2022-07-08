package server

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/bmharper/cyclops/server/dbh"
	"github.com/bmharper/cyclops/server/eventdb"
	"github.com/bmharper/cyclops/server/util"
)

func (s *Server) SetPermanentStoragePath(root string) error {
	if err := os.MkdirAll(root, 0777); err != nil {
		return fmt.Errorf("Failed to set permanent storage path '%v': %w", root, err)
	}

	s.Log.Infof("Permanent storage path '%v'", root)
	dbPath := filepath.Join(root, "permanent.sqlite")
	eventDB, err := dbh.OpenDB(s.Log, dbh.DriverSqlite, dbPath, eventdb.Migrations(s.Log), 0)
	if err == nil {
		s.permanentEventDB = eventDB
	} else {
		err = fmt.Errorf("Failed to open database %v: %w", dbPath, err)
	}
	return err
}

func (s *Server) SetRecentEventStoragePath(root string) error {
	if err := os.MkdirAll(root, 0777); err != nil {
		return fmt.Errorf("Failed to set event storage path '%v': %w", root, err)
	}

	s.Log.Infof("Recent event storage path '%v'", root)
	dbPath := filepath.Join(root, "event.sqlite")
	eventDB, err := dbh.OpenDB(s.Log, dbh.DriverSqlite, dbPath, eventdb.Migrations(s.Log), 0)
	if err == nil {
		s.eventDB = eventDB
	} else {
		err = fmt.Errorf("Failed to open database %v: %w", dbPath, err)
	}
	return err
}

// We don't want temp files to be on the videos dir, because the videos are likely to be
// stored on a USB flash drive, and this could cause the temp file to get written to disk,
// when we don't actually want that. We just want it as swap space... i.e. only written to disk
// if we run out of RAM.
func (s *Server) SetTempFilePath(tempFilePath string) error {
	s.Log.Infof("Temp file path '%v'", tempFilePath)
	if tempFiles, err := util.NewTempFiles(tempFilePath); err != nil {
		return err
	} else {
		s.TempFiles = tempFiles
	}
	return nil
}
