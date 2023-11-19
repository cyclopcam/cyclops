package video

import (
	"fmt"
	"io"
)

// Open a video file as a randomly seekable stream.
// If our storage is a blob store, then we use the StorageCache to cache the file locally.
// If our storage is a filesystem, then we return the file directly.
func (s *VideoServer) getSeekableVideoFile(vid int64, subfile string) (io.ReadSeekCloser, error) {
	if s.storageCache != nil {
		// Assume that the underlying storage system is a blob store that is a PITA to randomly seek
		// Instead of a cache, we could also use signed URLs (https://cloud.google.com/storage/docs/access-control/signing-urls-with-helpers#storage-signed-url-object-go)
		return s.storageCache.Open(videoFilename(vid, subfile))
	} else {
		file, err := s.storage.ReadFile(videoFilename(vid, subfile))
		if err != nil {
			return nil, err
		}
		if seeker, ok := file.Reader.(io.ReadSeekCloser); ok {
			return seeker, nil
		}
		return nil, fmt.Errorf("Underlying storage system does not support random seeking, and storage cache is not available")
	}
}

func (s *VideoServer) getLocalVideoFile(vid int64, subfile string) (io.Closer, string, error) {
	if s.storageCache != nil {
		file, err := s.storageCache.Open(videoFilename(vid, subfile))
		if err != nil {
			return nil, "", err
		}
		return file, file.Filename(), nil
	} else {
		filename, err := s.storage.Filename(videoFilename(vid, subfile))
		if err != nil {
			return nil, "", err
		}
		return io.NopCloser(nil), filename, nil
	}
}
