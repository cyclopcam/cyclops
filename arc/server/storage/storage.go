package storage

import (
	"errors"
	"io"
	"time"
)

var ErrNoPublicUrl = errors.New("No public URL available for this file")
var ErrNotAFilesystem = errors.New("This items in this blob store can't be accessed as local files")

// Storage is an abstraction of a blob store (eg S3), or a filesystem
type Storage interface {
	// When finished, you must close the WriteCloser
	WriteFile(name string) (io.WriteCloser, error)

	// When finished, you must close File.Reader
	ReadFile(name string) (*File, error)

	DeleteFile(name string) error

	// Return a URL to the given file. If that is not possible, return ErrNoPublicUrl.
	URL(name string) (string, error)

	// If this is a local filesystem, then return the local path to the file.
	Filename(name string) (string, error)
}

// File is an element in blob storage.
type File struct {
	Reader     io.ReadCloser
	ModifiedAt time.Time
	Size       int64
}

func WriteFile(s Storage, name string, content io.Reader) error {
	f, err := s.WriteFile(name)
	if err != nil {
		return err
	}
	_, err = io.Copy(f, content)
	errClose := f.Close()
	if err != nil {
		return err
	}
	return errClose
}

func ReadFile(s Storage, name string) ([]byte, error) {
	f, err := s.ReadFile(name)
	if err != nil {
		return nil, err
	}
	defer f.Reader.Close()
	return io.ReadAll(f.Reader)
}
