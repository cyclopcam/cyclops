package videodb

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/cyclopcam/cyclops/pkg/videoformat/fsv"
	"github.com/cyclopcam/dbh"
	"github.com/cyclopcam/logs"
	"gorm.io/gorm"
)

// VideoDB manages recordings
type VideoDB struct {
	// Root directory
	// root/fsv/...         Video file archive
	// root/videos.sqlite   Our SQLite DB
	Root string

	Archive *fsv.Archive

	log                   logs.Log
	db                    *gorm.DB
	shutdown              chan bool // This channel is closed when its time to shutdown
	writeThreadClosed     chan bool // The write thread closes this channel when it exits
	tileWriteThreadClosed chan bool // The tile write thread closes this channel when it exits
	maxClassesPerTile     int       // Max number of classes that we'll store in a tile
	debugTileLevelBuild   bool      // Emit extra logs
	debugTileWriter       bool

	// At level 13, each pixel is 8192 seconds. So a 2000 pixel screen is
	// 8192 * 2000 seconds = 190 days.
	maxTileLevel int

	// Objects that we are currently observing
	currentLock sync.Mutex
	current     map[uint32]*TrackedObject

	// This is a cache of the 'strings' table in the DB
	// Guards access to stringTableLock
	stringTableLock sync.Mutex
	stringToID      map[string]uint32 // MUST BE HOLDING stringTableLock. In-memory cache of the database table 'strings'. Use StringToID() to access this.
	idToString      map[uint32]string // MUST BE HOLDING stringTableLock. In-memory cache of the database table 'strings'. Use IDToString() to access this.

	// Tiles that we are building in real-time
	currentTilesLock sync.Mutex
	currentTiles     map[uint32][][]*tileBuilder // Key of the map is CameraID. Conceptually: currentTiles[CameraID][Level][TileIdx], although TileIdx is not a literal index into the slice.
}

// Open or create a video DB
func NewVideoDB(logger logs.Log, root string) (*VideoDB, error) {
	logsRaw := logger
	logger = logs.NewPrefixLogger(logsRaw, "VideoDB")

	root = filepath.Clean(root)
	if err := os.MkdirAll(root, 0770); err != nil {
		return nil, fmt.Errorf("Failed to create Video DB storage path '%v': %w", root, err)
	}

	videoDir := filepath.Join(root, "fsv")
	if err := os.Mkdir(videoDir, 0770); err != nil && !errors.Is(err, os.ErrExist) {
		return nil, fmt.Errorf("Failed to create Video storage path '%v': %w", videoDir, err)
	}

	logger.Infof("Opening Video DB at '%v'", root)
	dbPath := filepath.Join(root, "videos.sqlite")
	vdb, err := dbh.OpenDB(logger, dbh.MakeSqliteConfig(dbPath), Migrations(logger), dbh.DBConnectFlagSqliteWAL)
	if err != nil {
		return nil, fmt.Errorf("Failed to open video database %v: %w", dbPath, err)
	}

	// NOTE: I'm uneasy about us using default settings here (notably max archive size),
	// and then only later having SetMaxArchiveSize() called. We are safe, because the
	// default size limit is 0, which means "no limit". But it would feel better to
	// open the archive with our current settings, in case other settings creep in
	// later, and we don't remember to update that kind of thing here.

	logger.Infof("Scanning Video Archive at '%v'", videoDir)
	formats := []fsv.VideoFormat{&fsv.VideoFormatRF1{}}
	archiveInitSettings := fsv.DefaultStaticSettings()
	// The following line disables the write buffer
	//archiveInitSettings.MaxWriteBufferSize = 0
	fsvSettings := fsv.DefaultDynamicSettings()
	archive, err := fsv.Open(logsRaw, videoDir, formats, archiveInitSettings, fsvSettings)
	if err != nil {
		return nil, fmt.Errorf("Failed to open video archive at %v: %w", videoDir, err)
	}

	// At level 13, each pixel is 8192 seconds. So a 2000 pixel screen is
	// 8192 * 2000 seconds = 190 days.
	// SYNC-MAX-TILE-LEVEL
	maxTileLevel := 13

	self := &VideoDB{
		log:                   logger,
		db:                    vdb,
		Archive:               archive,
		Root:                  root,
		shutdown:              make(chan bool),
		writeThreadClosed:     make(chan bool),
		tileWriteThreadClosed: make(chan bool),
		maxClassesPerTile:     30, // Arbitrary constant to prevent terrible performance in pathological cases
		maxTileLevel:          maxTileLevel,
		current:               map[uint32]*TrackedObject{},
		stringToID:            map[string]uint32{},
		idToString:            map[uint32]string{},
		currentTiles:          map[uint32][][]*tileBuilder{},
		debugTileWriter:       false,
		debugTileLevelBuild:   true,
	}

	// Now that we write tiles of all levels at a regular interval, fillMissingTiles() is no longer needed.
	//self.fillMissingTiles(time.Now())
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

func (v *VideoDB) MaxTileLevel() int {
	return v.maxTileLevel
}

// tx may be nil, in which case we execute this statement outside of a transaction
func (v *VideoDB) setKV(key string, value any, tx *gorm.DB) error {
	if tx == nil {
		tx = v.db
	}
	err := tx.Exec("INSERT INTO kv (key, value) VALUES ($1, $2) ON CONFLICT(key) DO UPDATE SET value = excluded.value", key, value).Error
	if err != nil {
		v.log.Errorf("Failed to set KV %v: %v", key, err)
	}
	return err
}

// tx may be nil, in which case we execute this statement outside of a transaction
func (v *VideoDB) getKV(key string, dest any, tx *gorm.DB) error {
	if tx == nil {
		tx = v.db
	}
	return v.db.Raw("SELECT value FROM kv WHERE key = $1", key).Scan(dest).Error
}
