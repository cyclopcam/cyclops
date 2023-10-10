package storage

import (
	"io"
	"time"
)

// Storage is an abstraction of a blob store (eg S3)
type Storage interface {
	// When finished, you must close the WriteCloser
	WriteFile(name string) (io.WriteCloser, error)

	// When finished, you must close File.Reader
	ReadFile(name string) (*File, error)

	DeleteFile(name string) error
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
