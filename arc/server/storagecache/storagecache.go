package storagecache

import (
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/cyclopcam/cyclops/arc/server/storage"
	"github.com/cyclopcam/cyclops/pkg/log"
)

// NOTE: This unit is very poorly tested, because I started using
// publicly accessible URLs pretty early on.

// StorageCache caches blob store files on the local disk so that
// clients can seek inside them. It would be possible to build this
// functionality without a cache, but then I couldn't just use
// http.ServeContent, and would instead need to implement all of
// the range request headers, etc.
// See this issue for a discussion and example code that gets
// around this:
// https://github.com/googleapis/google-cloud-go/issues/1124
// The above solution is not ideal, because every Read() needs
// to re-open the blob. As far as I know, http.ServeContent
// is going to read in chunks of like 4k, so that would be
// terribly inefficient for 16MB file.
type StorageCache struct {
	log       log.Log
	upstream  storage.Storage
	cacheRoot string
	maxBytes  int64

	itemsLock sync.Mutex
	bytesUsed int64
	items     map[string]*cacheItem
	tick      int64
}

type cacheItem struct {
	filename string
	size     int64
	lock     int
	lastUsed int64
}

type CacheItemReader struct {
	store *StorageCache
	item  *cacheItem
	f     io.ReadSeekCloser // OS file in our cache
}

func (r *CacheItemReader) Read(p []byte) (n int, err error) {
	return r.f.Read(p)
}

func (r *CacheItemReader) Seek(offset int64, whence int) (int64, error) {
	return r.f.Seek(offset, whence)
}

func (r *CacheItemReader) Close() error {
	r.store.itemsLock.Lock()
	r.item.lock--
	defer r.store.itemsLock.Unlock()
	return r.f.Close()
}

func (r *CacheItemReader) Filename() string {
	return r.item.filename
}

func NewStorageCache(log log.Log, upstream storage.Storage, cacheRoot string, maxBytes int64) (*StorageCache, error) {
	os.RemoveAll(cacheRoot)
	if err := os.MkdirAll(cacheRoot, 0755); err != nil {
		return nil, err
	}
	c := &StorageCache{
		log:       log,
		upstream:  upstream,
		cacheRoot: cacheRoot,
		maxBytes:  maxBytes,
		items:     map[string]*cacheItem{},
	}
	return c, nil
}

func (s *StorageCache) Open(filename string) (*CacheItemReader, error) {
	s.itemsLock.Lock()
	defer s.itemsLock.Unlock()
	item := s.items[filename]
	if item == nil {
		s.purgeStale()
		if err := s.acquire(filename); err != nil {
			return nil, err
		}
		item = s.items[filename]
	}
	f, err := os.Open(filepath.Join(s.cacheRoot, filename))
	if err != nil {
		return nil, err
	}
	item.lock++
	item.lastUsed = s.tick
	s.tick++
	return &CacheItemReader{
		store: s,
		item:  item,
		f:     f,
	}, nil
}

func (s *StorageCache) acquire(filename string) error {
	src, err := s.upstream.ReadFile(filename)
	if err != nil {
		return err
	}
	defer src.Reader.Close()
	ondiskFilename := filepath.Join(s.cacheRoot, filename)
	if err := os.MkdirAll(filepath.Dir(ondiskFilename), 0755); err != nil {
		return err
	}
	dst, err := os.Create(ondiskFilename)
	if err != nil {
		return err
	}
	_, err = io.Copy(dst, src.Reader)
	if err == nil {
		err = dst.Close()
	} else {
		dst.Close()
	}
	if err != nil {
		os.Remove(dst.Name())
		return err
	}
	item := &cacheItem{
		filename: filename,
		size:     src.Size,
		lastUsed: s.tick,
		lock:     0,
	}
	s.bytesUsed += src.Size
	s.items[filename] = item
	return nil
}

func (s *StorageCache) purgeStale() {
	if s.bytesUsed > s.maxBytes {
		unused := []*cacheItem{}
		for _, item := range s.items {
			if item.lock == 0 {
				unused = append(unused, item)
			}
		}
		sort.Slice(unused, func(i, j int) bool {
			return unused[i].lastUsed < unused[j].lastUsed
		})
		for _, item := range unused {
			if s.bytesUsed <= s.maxBytes {
				break
			}
			s.bytesUsed -= item.size
			delete(s.items, item.filename)
			os.Remove(filepath.Join(s.cacheRoot, item.filename))
		}
	}
}
