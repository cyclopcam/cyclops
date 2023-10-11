package storage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/cyclopcam/cyclops/pkg/log"
)

// StorageFS is a filesystem-based blob store
type StorageFS struct {
	Root string
	log  log.Log
}

func NewStorageFS(log log.Log, root string) (*StorageFS, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(absRoot, 0755); err != nil {
		return nil, fmt.Errorf("Failed to create root directory %v (relative path %v): %w", absRoot, root, err)
	}
	return &StorageFS{
		Root: absRoot,
		log:  log,
	}, nil
}

func (fs *StorageFS) WriteFile(name string) (io.WriteCloser, error) {
	if strings.Index(name, "..") >= 0 {
		return nil, fmt.Errorf("Invalid file name %v", name)
	}
	fs.log.Infof("Writing file %v", name)
	fullPath := filepath.Join(fs.Root, name)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return nil, err
	}
	return os.OpenFile(fullPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
}

func (fs *StorageFS) ReadFile(name string) (*File, error) {
	if strings.Index(name, "..") >= 0 {
		return nil, fmt.Errorf("Invalid file name %v", name)
	}
	file, err := os.Open(filepath.Join(fs.Root, name))
	if err != nil {
		return nil, err
	}
	st, err := file.Stat()
	if err != nil {
		return nil, err
	}
	return &File{
		Reader:     file,
		ModifiedAt: st.ModTime(),
		Size:       st.Size(),
	}, nil
}

func (fs *StorageFS) DeleteFile(name string) error {
	if strings.Index(name, "..") >= 0 {
		return fmt.Errorf("Invalid file name %v", name)
	}
	fs.log.Infof("Deleting file %v", name)
	return os.Remove(filepath.Join(fs.Root, name))
}

func (fs *StorageFS) URL(name string) (string, error) {
	return "", ErrNoPublicUrl
}
