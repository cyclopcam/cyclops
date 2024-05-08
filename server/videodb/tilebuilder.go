package videodb

import (
	"errors"
	"time"
)

var ErrInvalidTimeRange = errors.New("invalid time range in tileBuilder.updateObject")
var ErrTooManyClasses = errors.New("too many classes")
var ErrNoTime = errors.New("no time data in TrackedObject for tileBuilder.updateObject")

// Number of pixels in on tile.
// At the highest resolution (level = 0), each pixel is 1 second.
const TileWidth = 1024

// A line of a bitmap.
// There is one line per class that we have detected (eg 'person' gets a line, 'car' gets a line, etc).
// Each bit on a line represents whether an object was detected during that interval (1 second for highest resolution).
type bitmapLine [TileWidth / 8]uint8

func (b *bitmapLine) setBit(i uint32) {
	b[i/8] |= 1 << (i % 8)
}

func (b *bitmapLine) getBit(i uint32) bool {
	return b[i/8]&(1<<(i%8)) != 0
}

func (b *bitmapLine) clear() {
	for i := range b {
		b[i] = 0
	}
}

// Sets all bits in the range [start, end) to 1.
func (b *bitmapLine) setBitRange(start, end uint32) {
	// handle start and end odd bits, and fill middle with byte writes
	if start > end || end > uint32(len(b))*8 {
		panic("setBitRange: out of bounds")
	}
	firstWholeByte := (start + 7) / 8
	lastWholeByte := end / 8
	if firstWholeByte*8 > start {
		startCap := min(firstWholeByte*8, end)
		for i := start; i < startCap; i++ {
			b.setBit(i)
		}
	}
	for i := firstWholeByte; i < lastWholeByte; i++ {
		b[i] = 0xff
	}
	if lastWholeByte*8 < end && firstWholeByte <= lastWholeByte {
		endCap := max(lastWholeByte*8, start)
		for i := endCap; i < end; i++ {
			b.setBit(i)
		}
	}
}

func (b *bitmapLine) formatRange(start, end int) string {
	s := make([]byte, 0, end-start)
	for i := uint32(start); i < uint32(end); i++ {
		if b.getBit(i) {
			s = append(s, '1')
		} else {
			s = append(s, '0')
		}
	}
	return string(s)
}

type tileBuilderTrackedObject struct {
	startTime uint32
	endTime   uint32
}

type tileBuilder struct {
	classes    map[uint32]*bitmapLine
	objects    map[uint32]tileBuilderTrackedObject
	maxClasses int
	baseTime   time.Time
}

func newTileBuilder(baseTime time.Time, maxClasses int) *tileBuilder {
	return &tileBuilder{
		classes:    make(map[uint32]*bitmapLine),
		objects:    make(map[uint32]tileBuilderTrackedObject),
		maxClasses: maxClasses,
		baseTime:   baseTime,
	}
}

func (b *tileBuilder) updateObject(obj *TrackedObject) error {
	// Take the max of obj.LastSeen and the time of the most recently added Box
	firstSeen := obj.LastSeen
	lastSeen := obj.LastSeen
	if len(obj.Boxes) != 0 {
		lastBox := obj.Boxes[len(obj.Boxes)-1]
		if lastBox.Time.After(lastSeen) {
			lastSeen = lastBox.Time
		}
		firstSeen = obj.Boxes[0].Time
	}
	t0f := firstSeen.Sub(b.baseTime).Seconds()
	t1f := lastSeen.Sub(b.baseTime).Seconds()
	t0f = max(t0f, 0)
	t1f = min(t1f, TileWidth-1)
	t0 := uint32(t0f)
	t1 := uint32(t1f)
	// ignore invalid ranges
	if t0 > t1 || t0 >= TileWidth {
		return ErrInvalidTimeRange
	}
	line, err := b.getBitmapForClass(obj.Class)
	if err != nil {
		return err
	}
	prev, ok := b.objects[obj.ID]
	if ok {
		// We've seen this object before.
		// Expand the bitmap from our previous startTime/endTime, to the current
		// startTime/endTime. It's very unlikely that startTime will change, and in
		// our current design it won't, but we might as well make allowance for it.
		if t0 < prev.startTime {
			line.setBitRange(t0, prev.startTime+1)
			prev.startTime = t0
		}
		if t1 > prev.endTime {
			line.setBitRange(prev.endTime+1, t1+1)
			prev.endTime = t1
		}
	} else {
		prev.startTime = t0
		prev.endTime = t1
		line.setBitRange(t0, t1+1)
	}
	b.objects[obj.ID] = prev
	return nil
}

// The only error that this function can return is ErrTooManyClasses
func (b *tileBuilder) getBitmapForClass(cls uint32) (*bitmapLine, error) {
	bmp := b.classes[cls]
	if bmp != nil {
		return bmp, nil
	}
	if len(b.classes) >= b.maxClasses {
		return nil, ErrTooManyClasses
	}
	bmp = &bitmapLine{}
	b.classes[cls] = bmp
	return bmp, nil
}
