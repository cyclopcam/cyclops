package server

import (
	"github.com/bmharper/cyclops/pkg/log"
	"github.com/bmharper/cyclops/server/eventdb"
	"github.com/bmharper/cyclops/server/util"
)

func (s *Server) SetPermanentStoragePath(root string) error {
	if db, err := eventdb.Open(log.NewPrefixLogger(s.Log, "PermEventDB"), root); err != nil {
		return err
	} else {
		s.permanentEvents = db
		return nil
	}
}

func (s *Server) SetRecentEventStoragePath(root string) error {
	if db, err := eventdb.Open(log.NewPrefixLogger(s.Log, "RecentEventDB"), root); err != nil {
		return err
	} else {
		s.recentEvents = db
		return nil
	}
}

// We don't want temp files to be on the videos dir, because the videos are likely to be
// stored on a USB flash drive, and this could cause the temp file to get written to disk,
// when we don't actually want that. We just want it as swap space... i.e. only written to disk
// if we run out of RAM.
func (s *Server) SetTempFilePath(tempFilePath string) error {
	s.Log.Infof("Temp file path '%v'", tempFilePath)
	if tempFiles, err := util.NewTempFiles(tempFilePath, s.Log); err != nil {
		return err
	} else {
		s.TempFiles = tempFiles
	}
	return nil
}
