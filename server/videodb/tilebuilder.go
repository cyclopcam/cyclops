package videodb

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math/bits"
	"slices"
	"time"

	"github.com/cyclopcam/cyclops/pkg/gen"
	"github.com/cyclopcam/cyclops/pkg/mybits"
)

var ErrInvalidTimeRange = errors.New("invalid time range in tileBuilder.updateObject")
var ErrTooManyClasses = errors.New("too many classes")
var ErrNoTime = errors.New("no time data in TrackedObject for tileBuilder.updateObject")

// Number of pixels in on tile.
// At the highest resolution (level = 0), each pixel is 1 second.
const TileWidth = 1024

// See point of use for explanation. Must be less than TileWidth/8 (aka 127 or less)
const maxOnOffEncodedLineBytes = 120

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

// A special optimization function for bitmap downsampling
// getBitPairOR(i) is equivalent to getBit(i) || getBit(i+1)
func (b *bitmapLine) getBitPairOR(i uint32) bool {
	return b[i/8]&(3<<(i%8)) != 0
}

func (b *bitmapLine) clear() {
	for i := range b {
		b[i] = 0
	}
}

// Sets all bits in the range [start, end) to 1.
// nBitsChanged is an optional output parameter. If not nil, it will be incremented by the number of changed bits.
func (b *bitmapLine) setBitRange(start, end uint32, nBitsChanged *int) {
	// handle start and end odd bits, and fill middle with byte writes
	if start > end || end > uint32(len(b))*8 {
		panic(fmt.Sprintf("setBitRange: out of bounds: start=%v, end=%v", start, end))
	}
	firstWholeByte := (start + 7) / 8
	lastWholeByte := end / 8
	if firstWholeByte*8 > start {
		startCap := min(firstWholeByte*8, end)
		for i := start; i < startCap; i++ {
			if nBitsChanged != nil && !b.getBit(i) {
				*nBitsChanged++
			}
			b.setBit(i)
		}
	}
	for i := firstWholeByte; i < lastWholeByte; i++ {
		if nBitsChanged != nil && b[i] != 0xff {
			*nBitsChanged += 8 - bits.OnesCount8(b[i])
		}
		b[i] = 0xff
	}
	if lastWholeByte*8 < end && firstWholeByte <= lastWholeByte {
		endCap := max(lastWholeByte*8, start)
		for i := endCap; i < end; i++ {
			if nBitsChanged != nil && !b.getBit(i) {
				*nBitsChanged++
			}
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
	classes               map[uint32]*bitmapLine
	objects               map[uint32]tileBuilderTrackedObject
	maxClasses            int // We'll refuse to add more classes once we hit this limit
	baseTime              time.Time
	tileIdx               uint32 // See timeToTileIdx()
	level                 uint32
	updateTick            int64 // Incremented whenever we write bits into the tile. Used with dbTick to determine if a tile needs to be written to disk.
	updateTickAtLastWrite int64 // Value of updateTick when we last wrote this tile to disk
}

func newTileBuilder(level uint32, baseTime time.Time, maxClasses int) *tileBuilder {
	seconds := baseTime.Unix()
	if (seconds>>level)%TileWidth != 0 {
		panic("baseTime of tile must be a multiple of TileWidth")
	}
	return &tileBuilder{
		classes:               make(map[uint32]*bitmapLine),
		objects:               make(map[uint32]tileBuilderTrackedObject),
		maxClasses:            maxClasses,
		baseTime:              baseTime,
		tileIdx:               timeToTileIdx(baseTime, level),
		level:                 level,
		updateTick:            0,
		updateTickAtLastWrite: 0,
	}
}

// Create a deep copy of the tileBuilder.
func (b *tileBuilder) clone() *tileBuilder {
	clone := newTileBuilder(b.level, b.baseTime, b.maxClasses)
	clone.updateTick = b.updateTick
	for k, v := range b.classes {
		lineCopy := &bitmapLine{}
		copy(lineCopy[:], v[:])
		clone.classes[k] = lineCopy
	}
	for k, v := range b.objects {
		clone.objects[k] = v
	}
	return clone
}

func (b *tileBuilder) isEmpty() bool {
	return len(b.classes) == 0
}

func (b *tileBuilder) updateObject(obj *TrackedObject) error {
	// Take the max of obj.LastSeen and the time of the most recently added Box
	firstSeen, lastSeen := obj.TimeBounds()
	factor := float64(uint32(1) << b.level)
	t0f := firstSeen.Sub(b.baseTime).Seconds() / factor
	t1f := lastSeen.Sub(b.baseTime).Seconds() / factor
	if t0f >= TileWidth {
		// This implies a logic error to reach this point
		return ErrInvalidTimeRange
	}
	//t0f = max(t0f, 0)
	//t1f = min(t0f, TileWidth-1)
	t0f = gen.Clamp(t0f, 0, TileWidth-1)
	t1f = gen.Clamp(t1f, 0, TileWidth-1)
	t0 := uint32(t0f)
	t1 := uint32(t1f)
	// ignore invalid ranges
	//if t0 > t1 || t0 >= TileWidth {
	if t0 > t1 {
		//return ErrInvalidTimeRange
		return fmt.Errorf("%w: %v %v -> %v %v", ErrInvalidTimeRange, t0f, t1f, t0, t1)
	}
	line, err := b.getBitmapForClass(obj.Class)
	if err != nil {
		return err
	}
	prev, ok := b.objects[obj.ID]
	nChangedBits := 0
	if ok {
		// We've seen this object before.
		// Expand the bitmap from our previous startTime/endTime, to the current
		// startTime/endTime. It's very unlikely that startTime will change, and in
		// our current design it won't, but we might as well make allowance for it.
		// Possible reasons for startTime going backwards would be an NN detection
		// that completed on a past frame.
		if t0 < prev.startTime {
			line.setBitRange(t0, prev.startTime+1, &nChangedBits)
			prev.startTime = t0
		}
		if t1 > prev.endTime {
			line.setBitRange(prev.endTime+1, t1+1, &nChangedBits)
			prev.endTime = t1
		}
	} else {
		prev.startTime = t0
		prev.endTime = t1
		line.setBitRange(t0, t1+1, &nChangedBits)
	}
	b.objects[obj.ID] = prev
	if nChangedBits != 0 {
		b.updateTick++
	}
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
	orderedClasses := []uint32{}
	for cls := range b.classes {
		orderedClasses = append(orderedClasses, cls)
	}
	// It's not necessary to sort by class, but I prefer it for consistency,
	// and it makes testing easier.
	// What IS necessary is to gather the classes into an array before iterating over them.
	// There was a bug in here initially where we'd iterate over b.classes twice, and when
	// those two iterations produced different ordering, we'd obviously end up with the
	// wrong bitmap assigned to the wrong class. I didn't know that Go map iteration is random
	// from iteration to iteration.
	slices.Sort(orderedClasses)

	blob := make([]byte, 0, 10+len(b.classes)*64)             // make slice capacity large enough to hold 50% compressed
	blob = binary.AppendUvarint(blob, 1)                      // version (v1 implies tile width 1024)
	blob = binary.AppendUvarint(blob, uint64(len(b.classes))) // number of classes
	for _, cls := range orderedClasses {
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
	// Our on/off encoding can bloat terribly (4x raw size) for pathological inputs
	// such as a bit pattern of 1010101010101. So we need a fallback to raw output,
	// which is exactly 128 bytes per line.

	for _, cls := range orderedClasses {
		// Limit the size to maxOnOffEncodedLineBytes. We could go all the way up to 127.
		// I don't know what's optimal here.
		line := b.classes[cls]
		encoded := [maxOnOffEncodedLineBytes]byte{}
		encodedLen, err := mybits.EncodeOnoff(line[:], encoded[:])
		if err != nil {
			// If we can't compress the line, then just write it raw.
			blob = append(blob, TileWidth/8)
			blob = append(blob, line[:]...)
		} else {
			// We know this length is maxOnOffEncodedLineBytes or less, and that's what tells the decoder that this line is compressed
			blob = append(blob, byte(encodedLen))
			blob = append(blob, encoded[:encodedLen]...)
		}
		//compressed := rle.Compress(line[:])
		//blob = append(blob, compressed...)
	}
	return blob
}

type readBlobFlags int

const (
	readBlobFlagSkipBitmaps = 1 // Do not read the bitmaps. Built for extracting the class IDs only.
)

func readBlobIntoTileBuilder(tileIdx uint32, level uint32, blob []byte, maxClasses int, flags readBlobFlags) (tb *tileBuilder, err error) {
	// The binary.Uvarint functions will panic if they run out of buffer,
	// so we just have a giant recover() around all of our statements.
	defer func() {
		if r := recover(); r != nil {
			tb = nil
			err = fmt.Errorf("Buffer underflow reading tile blob: %v", r)
		}
	}()
	skipBitmaps := flags&readBlobFlagSkipBitmaps != 0
	version, n := binary.Uvarint(blob)
	if version != 1 {
		return nil, fmt.Errorf("Unknown tile version %v", version)
	}
	blob = blob[n:]
	numClasses64, n := binary.Uvarint(blob)
	blob = blob[n:]
	numClasses := int(numClasses64)
	if numClasses > maxClasses {
		return nil, fmt.Errorf("Too many classes (%v) in tile blob", numClasses)
	}
	tb = newTileBuilder(level, tileIdxToTime(tileIdx, level), maxClasses)
	classes := make([]uint32, numClasses)
	for i := 0; i < numClasses; i++ {
		cls, n := binary.Uvarint(blob)
		blob = blob[n:]
		if skipBitmaps {
			tb.classes[uint32(cls)] = nil
		} else {
			tb.classes[uint32(cls)] = &bitmapLine{}
		}
		classes[i] = uint32(cls)
	}
	if skipBitmaps {
		return tb, nil
	}

	const tileWidthBytes = TileWidth / 8
	for i := 0; i < numClasses; i++ {
		cls := classes[i]
		// Encoded length of line, which is either 128 for raw, or less-than-or-equal-to maxOnOffEncodedLineBytes for on/off
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

	return tb, nil
}

func timeToTileIdx(t time.Time, level uint32) uint32 {
	return uint32(t.Unix()/TileWidth) >> level
}

func tileIdxToTime(tileIdx uint32, level uint32) time.Time {
	return time.Unix(int64(tileIdx<<level)*TileWidth, 0)
}

// Merge two levelX tiles into a levelX+1 tile.
func mergeTileBlobs(newTileIdx, newLevel uint32, blobA, blobB []byte, maxClasses int) ([]byte, error) {
	var tbA, tbB *tileBuilder
	var err error
	if len(blobA) != 0 {
		tbA, err = readBlobIntoTileBuilder(newTileIdx*2, newLevel-1, blobA, maxClasses, 0)
		if err != nil {
			return nil, err
		}
	}
	if len(blobB) != 0 {
		tbB, err = readBlobIntoTileBuilder(newTileIdx*2+1, newLevel-1, blobB, maxClasses, 0)
		if err != nil {
			return nil, err
		}
	}
	merged, err := mergeTileBuilders(newTileIdx, newLevel, tbA, tbB, maxClasses)
	if err != nil {
		return nil, err
	}
	return merged.writeBlob(), nil
}

// Merge two levelX tiles into a levelX+1 tile.
func mergeTileBuilders(newTileIdx, newLevel uint32, tbA, tbB *tileBuilder, maxClasses int) (*tileBuilder, error) {
	merged := newTileBuilder(newLevel, tileIdxToTime(newTileIdx, newLevel), maxClasses)
	if tbA != nil {
		if err := mergeTileIntoParent(tbA, merged, 0); err != nil {
			return nil, err
		}
	}
	if tbB != nil {
		if err := mergeTileIntoParent(tbB, merged, TileWidth/2); err != nil {
			return nil, err
		}
	}
	return merged, nil
}

// Here we downsample a bitmap from 1024 pixels to 512 pixels.
// The downsampling is an OR operation - in other words if bit 0 or 1 is
// set in the input, then bit 0 is set in the output. Likwise, if bit
// 2 or 3 is set in the input, then bit 1 is set in the output.
// This can almost definitely be done more efficiently, but I'm just
// running out of motivation on this chunk of code.
func mergeTileIntoParent(src, dst *tileBuilder, offset uint32) error {
	for srcCls, srcLine := range src.classes {
		dstLine, err := dst.getBitmapForClass(srcCls)
		if err != nil {
			return err
		}
		for i := uint32(0); i < TileWidth/2; i++ {
			// getBitPairOR(i*2) is equivalent to getBit(i*2) || getBit(i*2+1)
			if srcLine.getBitPairOR(i * 2) {
				dstLine.setBit(i + offset)
			}
		}
	}
	return nil
}

// This is for debug/analysis, specifically to create an extract of raw lines so that we can test our
// bitmap compression codecs.
// Returns a list of 128 byte bitmaps
func DecompressTileToRawLines(blob []byte) [][]byte {
	tb, err := readBlobIntoTileBuilder(0, 0, blob, 1000, 0)
	if err != nil {
		panic(err)
	}
	lines := [][]byte{}
	for _, line := range tb.classes {
		lines = append(lines, line[:])
	}
	return lines
}

func init() {
	if maxOnOffEncodedLineBytes >= TileWidth/8 {
		panic("maxOnOffEncodedLineBytes must be less than TileWidth/8")
	}
}
