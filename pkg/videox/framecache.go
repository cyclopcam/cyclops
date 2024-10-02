package videox

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/cyclopcam/cyclops/pkg/accel"
)

// FrameCache is used to speed up the fetching of individual frames while
// a user is seeking around in a video.
// We cache YUV images.
type FrameCache struct {
	MaxMemory  int // Maximum bytes of RAM to use
	MemoryUsed int // Current bytes of RAM used

	lock   sync.Mutex
	frames map[string]*cachedFrame
}

// NewFrameCache creates a new FrameCache with the given maximum memory usage
func NewFrameCache(maxMemory int) *FrameCache {
	return &FrameCache{
		MaxMemory: maxMemory,
		frames:    make(map[string]*cachedFrame),
	}
}

type cachedFrame struct {
	key      string
	lastUsed time.Time
	frame    *accel.YUVImage
}

func (f *FrameCache) MakeKey(videoKey string, framePTSUnixMS int64) string {
	return fmt.Sprintf("%v-%v", videoKey, framePTSUnixMS)
}

// Return the frame or nil
func (f *FrameCache) GetFrame(key string) *accel.YUVImage {
	f.lock.Lock()
	defer f.lock.Unlock()
	frame := f.frames[key]
	if frame == nil {
		return nil
	}
	frame.lastUsed = time.Now()
	return frame.frame
}

// Add a frame to the cache
func (f *FrameCache) AddFrame(key string, frame *accel.YUVImage) {
	f.lock.Lock()
	defer f.lock.Unlock()
	if f.frames[key] != nil {
		// This happens quite frequently, because we always have to seek back to a keyframe to decode
		// a given future frame. So you'll run over the same set of frames multiple times, in order to
		// reach a future frame. We *do* try to minimize this though, by speculatively decoding ahead
		// of where we need to.
		return
	}
	f.autoEvict()
	f.frames[key] = &cachedFrame{
		key:      key,
		lastUsed: time.Now(),
		frame:    frame,
	}
	f.MemoryUsed += frame.TotalBytes()
}

// If we've blown our RAM budget, then evict the oldest frames until
// we're 10% under budget.
// You must be holding the lock before calling this function.
func (f *FrameCache) autoEvict() {
	if f.MemoryUsed < f.MaxMemory {
		return
	}
	allFrames := []*cachedFrame{}
	for _, v := range f.frames {
		allFrames = append(allFrames, v)
	}
	sort.Slice(allFrames, func(i, j int) bool {
		return allFrames[i].lastUsed.Before(allFrames[j].lastUsed)
	})
	for f.MemoryUsed > f.MaxMemory*9/10 {
		if len(allFrames) == 0 {
			return
		}
		frame := allFrames[0]
		delete(f.frames, frame.key)
		f.MemoryUsed -= frame.frame.TotalBytes()
		allFrames = allFrames[1:]
	}
}
