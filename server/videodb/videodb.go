package videodb

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cyclopcam/cyclops/pkg/dbh"
	"github.com/cyclopcam/cyclops/pkg/log"
	"github.com/cyclopcam/cyclops/pkg/videoformat/fsv"
	"gorm.io/gorm"
)

// VideoDB manages recordings
type VideoDB struct {
	Root    string // Where we store our videos (also directory where sqlite DB is stored)
	Archive *fsv.Archive

	log log.Log
	db  *gorm.DB
}

// Open or create a video DB
func NewVideoDB(log log.Log, root string) (*VideoDB, error) {
	root = filepath.Clean(root)
	if err := os.MkdirAll(root, 0770); err != nil {
		return nil, fmt.Errorf("Failed to create Video DB storage path '%v': %w", root, err)
	}

	videoDir := filepath.Join(root, "fsv")
	if err := os.Mkdir(videoDir, 0770); err != nil && !errors.Is(err, os.ErrExist) {
		return nil, fmt.Errorf("Failed to create Video storage path '%v': %w", videoDir, err)
	}

	log.Infof("Opening Video DB at '%v'", root)
	dbPath := filepath.Join(root, "videos.sqlite")
	vdb, err := dbh.OpenDB(log, dbh.MakeSqliteConfig(dbPath), Migrations(log), 0)
	if err != nil {
		return nil, fmt.Errorf("Failed to open video database %v: %w", dbPath, err)
	}

	log.Infof("Scanning Video Archive at '%v'", videoDir)
	formats := []fsv.VideoFormat{&fsv.VideoFormatRF1{}}
	fsvSettings := fsv.DefaultArchiveSettings()
	archive, err := fsv.Open(log, videoDir, formats, fsvSettings)
	if err != nil {
		return nil, fmt.Errorf("Failed to open video archive at %v: %w", videoDir, err)
	}

	self := &VideoDB{
		log:     log,
		db:      vdb,
		Archive: archive,
		Root:    root,
	}
	return self, nil
}

func (v *VideoDB) SetMaxArchiveSize(maxSize int64) {
	s := v.Archive.Settings()
	s.MaxArchiveSize = maxSize
	v.Archive.SetSettings(s)
}

func (v *VideoDB) Close() {
	v.Archive.Close()
}
