package videodb

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/cyclopcam/cyclops/pkg/dbh"
	"github.com/cyclopcam/cyclops/pkg/log"
	"github.com/cyclopcam/cyclops/pkg/videoformat/fsv"
	"gorm.io/gorm"
)

// VideoDB manages recordings
type VideoDB struct {
	// Root directory
	// root/fsv/...         Video file archive
	// root/videos.sqlite   Our SQLite DB
	Root string

	Archive *fsv.Archive

	log               log.Log
	db                *gorm.DB
	shutdown          chan bool // This channel is closed when its time to shutdown
	writeThreadClosed chan bool // The write thread closes this channel when it exits

	// Objects that we are currently observing
	currentLock sync.Mutex // Guards access to 'current'
	current     []*TrackedObject
}

// Open or create a video DB
func NewVideoDB(logs log.Log, root string) (*VideoDB, error) {
	logsRaw := logs
	logs = log.NewPrefixLogger(logs, "VideoDB")

	root = filepath.Clean(root)
	if err := os.MkdirAll(root, 0770); err != nil {
		return nil, fmt.Errorf("Failed to create Video DB storage path '%v': %w", root, err)
	}

	videoDir := filepath.Join(root, "fsv")
	if err := os.Mkdir(videoDir, 0770); err != nil && !errors.Is(err, os.ErrExist) {
		return nil, fmt.Errorf("Failed to create Video storage path '%v': %w", videoDir, err)
	}

	logs.Infof("Opening Video DB at '%v'", root)
	dbPath := filepath.Join(root, "videos.sqlite")
	vdb, err := dbh.OpenDB(logs, dbh.MakeSqliteConfig(dbPath), Migrations(logs), 0)
	if err != nil {
		return nil, fmt.Errorf("Failed to open video database %v: %w", dbPath, err)
	}

	logs.Infof("Scanning Video Archive at '%v'", videoDir)
	formats := []fsv.VideoFormat{&fsv.VideoFormatRF1{}}
	fsvSettings := fsv.DefaultArchiveSettings()
	archive, err := fsv.Open(logsRaw, videoDir, formats, fsvSettings)
	if err != nil {
		return nil, fmt.Errorf("Failed to open video archive at %v: %w", videoDir, err)
	}

	self := &VideoDB{
		log:               logs,
		db:                vdb,
		Archive:           archive,
		Root:              root,
		shutdown:          make(chan bool),
		writeThreadClosed: make(chan bool),
	}

	go self.eventWriteThread()

	return self, nil
}

// The archive won't delete any files until this is called, because it doesn't know yet
// what the size limit is.
func (v *VideoDB) SetMaxArchiveSize(maxSize int64) {
	s := v.Archive.Settings()
	s.MaxArchiveSize = maxSize
	v.Archive.SetSettings(s)
}

func (v *VideoDB) Close() {
	close(v.shutdown)
	v.log.Infof("Waiting for fsv archive to close")
	v.Archive.Close()
	v.log.Infof("Waiting for event write thread to exit")
	<-v.writeThreadClosed
}
