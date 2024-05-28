package videodb

import (
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	"github.com/cyclopcam/cyclops/pkg/mybits"
)

var ErrInvalidTimeRange = errors.New("invalid time range in tileBuilder.updateObject")
var ErrTooManyClasses = errors.New("too many classes")
var ErrNoTime = errors.New("no time data in TrackedObject for tileBuilder.updateObject")

// Number of pixels in on tile.
// At the highest resolution (level = 0), each pixel is 1 second.
const TileWidth = 1024

// See point of use for explanation
const maxOnOffEncodedLineBytes = 64

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
	maxClasses int // We'll refuse to add more classes once we hit this limit
	baseTime   time.Time
	tileIdx    uint32 // See timeToTileIdx()
	level      uint32
	isResume   bool // True if this tile is already in the database, and we're resuming the creation of it (due to a restart)
}

func newTileBuilder(level uint32, baseTime time.Time, maxClasses int) *tileBuilder {
	seconds := baseTime.Unix()
	if (seconds>>level)%TileWidth != 0 {
		panic("baseTime of tile must be a multiple of TileWidth")
	}
	return &tileBuilder{
		classes:    make(map[uint32]*bitmapLine),
		objects:    make(map[uint32]tileBuilderTrackedObject),
		maxClasses: maxClasses,
		baseTime:   baseTime,
		tileIdx:    timeToTileIdx(baseTime, level),
		level:      level,
		isResume:   false,
	}
}

func (b *tileBuilder) isEmpty() bool {
	return len(b.classes) == 0
}

func (b *tileBuilder) updateObject(obj *TrackedObject) error {
	// Take the max of obj.LastSeen and the time of the most recently added Box
	firstSeen, lastSeen := obj.TimeBounds()
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

func (b *tileBuilder) writeBlob() []byte {
	blob := make([]byte, 0, 10+len(b.classes)*64)             // large enough to hold 50% compressed
	blob = binary.AppendUvarint(blob, 1)                      // version (v1 implies tile width 1024)
	blob = binary.AppendUvarint(blob, uint64(len(b.classes))) // number of classes
	for cls := range b.classes {
		blob = binary.AppendUvarint(blob, uint64(cls))
	}
	// V0.1:
	// Our RLE codec is extremely simple, and it allows us to concatenate
	// compressed streams. So we can compress each class bitmap line independently,
	// and just lump all of the compressed bitmap lines together into one big chunk.
	// On decompression, we decompress them all into one big bitmap, and then split
	// them off line by line.
	// It's convenient to compress all lines into one big buffer, because then we
	// don't need to store N different values for the size of each compressed line.
	// The size of our compressed data is implicit.
	// It is sizeof(blob) - sizeof(all the other stuff, such as header & classes).

	// V1:
	// Compress every line with either on/off encoding or raw.
	// Our on/off encoding can bloat terribly (8x raw size) for pathological inputs
	// such as a bit pattern of 1010101010101. So we need a fallback to raw output,
	// which is exactly 128 bytes per line.

	for _, line := range b.classes {
		// Limit the size to 64 bytes. We could go all the way up to 127.
		// I don't know what's optimal here.
		encoded := [maxOnOffEncodedLineBytes]byte{}
		encodedLen, err := mybits.EncodeOnoff(line[:], encoded[:])
		if err != nil {
			// If we can't compress the line, then just write it raw.
			blob = append(blob, TileWidth/8)
			blob = append(blob, line[:]...)
		} else {
			// We know this length is 64 or less, and that's what tells the decoder that this line is compressed
			blob = append(blob, byte(encodedLen))
			blob = append(blob, encoded[:encodedLen]...)
		}
		//compressed := rle.Compress(line[:])
		//blob = append(blob, compressed...)
	}
	return blob
}

func readBlobIntoTileBuilder(tileIdx uint32, level uint32, blob []byte, maxClasses int) (tb *tileBuilder, err error) {
	// The binary.Uvarint functions will panic if they run out of buffer,
	// so we just have a giant recover() around all of our statements.
	defer func() {
		if r := recover(); r != nil {
			tb = nil
			err = fmt.Errorf("Buffer underflow reading tile blob: %v", r)
		}
	}()
	version, n := binary.Uvarint(blob)
	if version != 1 {
		return nil, fmt.Errorf("Unknown tile version %v", version)
	}
	blob = blob[n:]
	numClasses64, n := binary.Uvarint(blob)
	blob = blob[n:]
	numClasses := int(numClasses64)
	if numClasses > maxClasses*4 || numClasses > 1024 {
		// sanity check, to prevent us running out of memory in the face of a badly
		// formed tile blob.
		return nil, fmt.Errorf("Too many classes (%v) in tile blob", numClasses)
	}
	tb = newTileBuilder(level, tileIdxToTime(tileIdx, level), maxClasses)
	tb.isResume = true
	classes := make([]uint32, numClasses)
	for i := 0; i < numClasses; i++ {
		cls, n := binary.Uvarint(blob)
		blob = blob[n:]
		line := &bitmapLine{}
		tb.classes[uint32(cls)] = line
		classes[i] = uint32(cls)
	}
	const tileWidthBytes = TileWidth / 8
	for i := 0; i < numClasses; i++ {
		cls := classes[i]
		// Encoded length of line, which is either 128 for raw, or LTE maxOnOffEncodedLineBytes for on/off
		encLen := blob[0]
		blob = blob[1:]
		if encLen == byte(tileWidthBytes) {
			copy(tb.classes[cls][:], blob[:tileWidthBytes])
		} else if encLen <= maxOnOffEncodedLineBytes {
			rawBitLen, decErr := mybits.DecodeOnoff(blob[:encLen], tb.classes[cls][:])
			if decErr != nil {
				return nil, fmt.Errorf("While decoding bitmap line: %w", decErr)
			} else if rawBitLen != TileWidth {
				return nil, fmt.Errorf("Decompressed tile line is wrong size: %v (expected %v)", rawBitLen, TileWidth)
			}
		} else {
			return nil, fmt.Errorf("Invalid encoded line length %v", encLen)
		}
		blob = blob[encLen:]
	}

	/*
		lines := make([]byte, numClasses*tileWidthBytes)
		// V0.1 (RLE)
		rawLineBytes, decErr := rle.Decompress(blob, lines)
		if decErr != nil {
			return nil, decErr
		}
		if rawLineBytes != numClasses*tileWidthBytes {
			return nil, fmt.Errorf("Decompressed tile blob is wrong size: %v (expected %v)", rawLineBytes, numClasses*tileWidthBytes)
		}
		// Copy the decompressed lines from the single big decompression buffer,
		// into individual bitmapLine objects, in the tb 'classes' map.
		for i := 0; i < numClasses; i++ {
			cls := classes[i]
			copy(tb.classes[cls][:], lines[i*tileWidthBytes:(i+1)*tileWidthBytes])
		}
	*/
	return tb, nil
}

func timeToTileIdx(t time.Time, level uint32) uint32 {
	return uint32(t.Unix()/TileWidth) >> level
}

func tileIdxToTime(tileIdx uint32, level uint32) time.Time {
	return time.Unix(int64(tileIdx<<level)*TileWidth, 0)
}
