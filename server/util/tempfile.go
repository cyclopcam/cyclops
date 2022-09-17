package util

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/bmharper/cyclops/pkg/gen"
	"github.com/bmharper/cyclops/pkg/log"
)

// TempFiles assigns temporary filenames, and automatically deletes old temporary files.
// We don't need to be too worried about aggressive deletion of temp files, because if
// a temp file is still being sent to a client, and we delete it, then the OS will do
// the right thing and not actually erase the file until the last handle is closed.
//
// We manage two classes of temporary files:
//  1. Once off: These are intended to be created, used, and deleted.
//  2. Named: These are intended to be named with a hash, or something similarly unique.
//     Named files live longer. This was created to cache transcoded videos with improved
//     random seek behaviour in a browser <video> element. Naturally, we want this kind of
//     thing to stick around for a while.
type TempFiles struct {
	log   log.Log
	debug bool // add more logging

	lock                    sync.Mutex // guards access to all internal state
	pathOnceoff             string     // Root of once-off files
	pathNamed               string     // Root of named files
	lastCleanupOnceoff      time.Time
	lastCleanupNamed        time.Time
	maxAgeOnceoff           time.Duration
	maxAgeNamed             time.Duration
	guaranteedLifetimeNamed time.Duration
	hotNamed                map[string]time.Time // the moment when a named file was last requested (so we don't have to change mtime of files)
	lastHotCleanup          time.Time            // When hotNamed was last cleaned up
}

// Wipes/recreates the root directory
func NewTempFiles(root string, logger log.Log) (*TempFiles, error) {
	pathOnceoff := filepath.Join(root, "once")
	pathNamed := filepath.Join(root, "named")
	dirs := []string{
		pathOnceoff,
		pathNamed,
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0777); err != nil {
			return nil, fmt.Errorf("Failed to create temporary file directory '%v': %w", d, err)
		}
	}

	return &TempFiles{
		debug:                   false,
		log:                     log.NewPrefixLogger(logger, "TempFiles"),
		pathOnceoff:             pathOnceoff,
		pathNamed:               pathNamed,
		lastCleanupOnceoff:      time.Now(),
		lastCleanupNamed:        time.Now(),
		maxAgeOnceoff:           30 * time.Second,
		maxAgeNamed:             2 * time.Hour,
		guaranteedLifetimeNamed: time.Minute,
		hotNamed:                map[string]time.Time{},
		lastHotCleanup:          time.Now(),
	}, nil
}

// Get a new temporary filename for a once-off file
func (t *TempFiles) GetOnceOff() string {
	t.lock.Lock()
	defer t.lock.Unlock()
	if time.Now().Sub(t.lastCleanupOnceoff) > t.cleanupIntervalOf(t.maxAgeOnceoff) {
		t.lastCleanupOnceoff = time.Now()
		go t.cleanOld(t.pathOnceoff, t.maxAgeOnceoff, nil)
	}
	fullpath := fmt.Sprintf(filepath.Join(t.pathOnceoff, fmt.Sprintf("%d", time.Now().UnixNano())))
	if t.debug {
		t.log.Infof("Create once off: %v", fullpath)
	}
	return fullpath
}

// Get a temporary filename for a named item.
// If you ask for the same name twice, you'll get back the same path.
// This function will guarantee that the named file will not be deleted for at least
// guaranteedLifetimeNamed.
// If the file already exists, then 'exists' is true.
func (t *TempFiles) GetNamed(name string) (filename string, exists bool) {
	t.lock.Lock()
	defer t.lock.Unlock()
	now := time.Now()

	// Place file in "hot" hash table
	t.hotNamed[name] = now
	if now.Sub(t.lastHotCleanup) > t.guaranteedLifetimeNamed {
		t.lastHotCleanup = now
		t.cleanHot()
	}

	// Cleanup files on disk, if necessary
	if now.Sub(t.lastCleanupNamed) > t.cleanupIntervalOf(t.maxAgeNamed) {
		t.lastCleanupNamed = now
		go t.cleanOld(t.pathNamed, t.maxAgeNamed, gen.CopyMap(t.hotNamed))
	}

	filename = filepath.Join(t.pathNamed, name)
	if t.debug {
		t.log.Infof("Create named: %v", filename)
	}
	_, err := os.Stat(filename)
	exists = err == nil
	return
}

// Remove old files from the hotNamed map
func (t *TempFiles) cleanHot() {
	now := time.Now()
	old := []string{}
	for name, touched := range t.hotNamed {
		if now.Sub(touched) > t.guaranteedLifetimeNamed {
			old = append(old, name)
		}
	}
	for _, n := range old {
		if t.debug {
			t.log.Infof("Purge hot: %v", n)
		}
		delete(t.hotNamed, n)
	}
}

// This function must not touch any shared mutable state, or take the lock, because it can
// run on a background goroutine
func (t *TempFiles) cleanOld(dir string, maxAge time.Duration, touchedAt map[string]time.Time) {
	now := time.Now()

	if t.debug {
		t.log.Infof("Cleanup %v, max age %v seconds", dir, maxAge.Seconds())
	}

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if d == nil {
			// The entire walk failed
			return err
		}
		if err == nil && !d.IsDir() {
			lastTouch := time.Time{}
			if touchedAt != nil {
				// Use value from in-memory name:time hash table
				lastTouch = touchedAt[d.Name()]
			}
			if lastTouch.IsZero() {
				// Use filesystem ModTime as "last access time"
				if inf, err := d.Info(); err == nil {
					lastTouch = inf.ModTime()
				}
			}
			if now.Sub(lastTouch) > maxAge {
				if t.debug {
					t.log.Infof("Delete %v", path)
				}
				os.Remove(path)
			}
		} else if err != nil {
			t.log.Errorf("WalkDir on item %v failed: %v", path, err)
		}
		return nil
	})
	if err != nil {
		t.log.Errorf("WalkDir on directory %v failed: %v", dir, err)
	}
}

// For a given maximum age, return a reasonable cleanup interval
func (t *TempFiles) cleanupIntervalOf(maxAge time.Duration) time.Duration {
	return maxAge / 2
}
