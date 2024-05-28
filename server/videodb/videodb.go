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

	log                   log.Log
	db                    *gorm.DB
	shutdown              chan bool // This channel is closed when its time to shutdown
	writeThreadClosed     chan bool // The write thread closes this channel when it exits
	tileWriteThreadClosed chan bool // The tile write thread closes this channel when it exits
	maxClassesPerTile     int       // Max number of classes that we'll store in a tile

	// Objects that we are currently observing
	currentLock sync.Mutex
	current     map[uint32]*TrackedObject

	// Guards access to stringToIDLock
	stringToIDLock sync.Mutex
	stringToID     map[string]uint32 // In-memory cache of the database table 'strings'

	// Tiles that we are building in real-time
	currentTilesLock sync.Mutex
	currentTiles     map[uint32][]*tileBuilder // Key is CameraID
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

	// NOTE: I'm uneasy about us using default settings here (notably max archive size),
	// and then only later having SetMaxArchiveSize() called. We are safe, because the
	// default size limit is 0, which means "no limit". But it would feel better to
	// open the archive with our current settings, in case other settings creep in
	// later, and we don't remember to update that kind of thing here.

	logs.Infof("Scanning Video Archive at '%v'", videoDir)
	formats := []fsv.VideoFormat{&fsv.VideoFormatRF1{}}
	archiveInitSettings := fsv.DefaultStaticSettings()
	// The following line disables the write buffer
	//archiveInitSettings.MaxWriteBufferSize = 0
	fsvSettings := fsv.DefaultDynamicSettings()
	archive, err := fsv.Open(logsRaw, videoDir, formats, archiveInitSettings, fsvSettings)
	if err != nil {
		return nil, fmt.Errorf("Failed to open video archive at %v: %w", videoDir, err)
	}

	self := &VideoDB{
		log:                   logs,
		db:                    vdb,
		Archive:               archive,
		Root:                  root,
		shutdown:              make(chan bool),
		writeThreadClosed:     make(chan bool),
		tileWriteThreadClosed: make(chan bool),
		maxClassesPerTile:     30, // Arbitrary constant to prevent terrible performance in pathological cases
		current:               map[uint32]*TrackedObject{},
		stringToID:            map[string]uint32{},
		currentTiles:          map[uint32][]*tileBuilder{},
	}

	self.resumeLatestTiles()

	go self.eventWriteThread()
	go self.tileWriteThread()

	return self, nil
}

// The archive won't delete any files until this is called, because it doesn't know yet
// what the size limit is.
func (v *VideoDB) SetMaxArchiveSize(maxSize int64) {
	s := v.Archive.GetDynamicSettings()
	s.MaxArchiveSize = maxSize
	v.Archive.SetDynamicSettings(s)
}

func (v *VideoDB) Close() {
	close(v.shutdown)
	v.log.Infof("Waiting for fsv archive to close")
	v.Archive.Close()
	v.log.Infof("Waiting for event write thread to exit")
	<-v.writeThreadClosed
	v.log.Infof("Waiting for event summary write thread to exit")
	<-v.tileWriteThreadClosed
}
