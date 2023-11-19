package storage

import (
	"context"
	"io"

	gcs "cloud.google.com/go/storage"
	"github.com/cyclopcam/cyclops/pkg/log"
)

// StorageGCS is a Google Cloud Storage-based blob store
type StorageGCS struct {
	bucketName string
	bucket     *gcs.BucketHandle
	isPublic   bool
	log        log.Log
}

func NewStorageGCS(log log.Log, bucketName string, isPublic bool) (*StorageGCS, error) {
	ctx := context.Background()
	client, err := gcs.NewClient(ctx)
	if err != nil {
		return nil, err
	}
	bucket := client.Bucket(bucketName)
	return &StorageGCS{
		bucketName: bucketName,
		bucket:     bucket,
		isPublic:   isPublic,
		log:        log,
	}, nil
}

func (s *StorageGCS) WriteFile(name string) (io.WriteCloser, error) {
	ctx := context.Background()
	w := s.bucket.Object(name).NewWriter(ctx)
	return w, nil
}

func (s *StorageGCS) ReadFile(name string) (*File, error) {
	ctx := context.Background()
	r, err := s.bucket.Object(name).NewReader(ctx)
	if err != nil {
		return nil, err
	}
	return &File{
		Reader:     r,
		ModifiedAt: r.Attrs.LastModified,
		Size:       r.Attrs.Size,
	}, nil
}

func (s *StorageGCS) DeleteFile(name string) error {
	ctx := context.Background()
	return s.bucket.Object(name).Delete(ctx)
}

func (s *StorageGCS) URL(name string) (string, error) {
	if !s.isPublic {
		// We could also use signed URLs, but I haven't bothered with that yet
		return "", ErrNoPublicUrl
	}
	return "https://storage.googleapis.com/" + s.bucketName + "/" + name, nil
}

func (s *StorageGCS) Filename(name string) (string, error) {
	return "", ErrNotAFilesystem
}
