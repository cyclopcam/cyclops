package util

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

// TempFiles assigns temporary filenames, and automatically deletes old temporary files.
// We don't need to be too worried about aggressive deletion of temp files, because if
// a temp file is still being sent to a client, and we delete it, then the OS will do
// the right thing and not actually erase the file until the last handle is closed.
type TempFiles struct {
	Root string

	lock            sync.Mutex // guards access to all internal state
	lastCleanup     time.Time
	cleanupInterval time.Duration
	maxAge          time.Duration
}

// Wipes/recreates the root directory
func NewTempFiles(root string) (*TempFiles, error) {
	if err := os.MkdirAll(root, 0777); err != nil {
		return nil, fmt.Errorf("Failed to create temporary file directory '%v': %w", root, err)
	}

	all, _ := filepath.Glob(filepath.Join(root, "*"))
	for _, fn := range all {
		os.Remove(fn)
	}
	return &TempFiles{
		Root:            root,
		lastCleanup:     time.Now(),
		cleanupInterval: 1 * time.Minute,
		maxAge:          1 * time.Minute,
	}, nil
}

// Get a new temporary filename
func (t *TempFiles) Get() string {
	t.lock.Lock()
	defer t.lock.Unlock()
	if time.Now().Sub(t.lastCleanup) > t.cleanupInterval {
		t.lastCleanup = time.Now()
		go t.cleanOld()
	}
	return fmt.Sprintf(filepath.Join(t.Root, fmt.Sprintf("%d", time.Now().UnixNano())))
}

// this must not touch any shared mutable state, or take the lock
func (t *TempFiles) cleanOld() {
	all, _ := filepath.Glob(filepath.Join(t.Root, "*"))
	now := time.Now().UnixNano()
	threshold := t.maxAge.Nanoseconds()
	for _, fn := range all {
		createdAt, _ := strconv.ParseInt(fn, 10, 64)
		if now-createdAt > threshold {
			//fmt.Printf("Deleting old temp file %v\n", fn)
			os.Remove(fn)
		}
	}
}
