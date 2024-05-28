package videodb

import (
	"fmt"
	"testing"
	"time"

	"github.com/cyclopcam/cyclops/pkg/nn"
	"github.com/stretchr/testify/require"
)

func TestBitmapLine(t *testing.T) {
	b := bitmapLine{}
	b.setBitRange(0, 8)
	require.Equal(t, "11111111", b.formatRange(0, 8))

	b.clear()
	b.setBitRange(0, 16)
	require.Equal(t, "1111111111111111", b.formatRange(0, 16))

	b.clear()
	b.setBitRange(8, 16)
	require.Equal(t, "0000000011111111", b.formatRange(0, 16))

	b.clear()
	b.setBitRange(1, 8)
	require.Equal(t, "01111111", b.formatRange(0, 8))

	b.clear()
	b.setBitRange(1, 7)
	require.Equal(t, "01111110", b.formatRange(0, 8))

	b.clear()
	b.setBitRange(1, 10)
	require.Equal(t, "011111111100", b.formatRange(0, 12))

	b.clear()
	b.setBitRange(10, 12)
	require.Equal(t, "000000000011000", b.formatRange(0, 15))

	b.clear()
	b.setBitRange(7, 9)
	require.Equal(t, "0000000110", b.formatRange(0, 10))

	b.clear()
	b.setBitRange(2, 27)
	require.Equal(t, "00111111111111111111111111100000", b.formatRange(0, 32))
}

func TestTileBuilder1(t *testing.T) {
	base := tileIdxToTime(12345, 0)
	b := newTileBuilder(0, base, 100)
	obj := &TrackedObject{
		ID:       1,
		Camera:   99,
		Class:    123,
		LastSeen: base.Add(2500 * time.Millisecond),
	}
	obj.Boxes = append(obj.Boxes, TrackedBox{
		Time: base.Add(2500 * time.Millisecond),
		Box:  nn.MakeRect(5, 6, 13, 8),
	})
	b.updateObject(obj)
	require.Equal(t, "0010000000", b.classes[obj.Class].formatRange(0, 10))
	// I single-step into the following line and verify that no setBit() writes occur, for efficiency sake (not correctness).
	b.updateObject(obj)
	require.Equal(t, "0010000000", b.classes[obj.Class].formatRange(0, 10))

	// verify that LastSeen adds bits to the end
	obj.LastSeen = base.Add(4500 * time.Millisecond)
	b.updateObject(obj)
	require.Equal(t, "0011100000", b.classes[obj.Class].formatRange(0, 10))

	// verify that the tile builder uses max(LastSeen, Boxes[len(Boxes)-1].Time
	obj.Boxes = append(obj.Boxes, TrackedBox{
		Time: base.Add(5500 * time.Millisecond),
		Box:  nn.MakeRect(5, 6, 13, 8),
	})
	b.updateObject(obj)
	require.Equal(t, "0011110000", b.classes[obj.Class].formatRange(0, 10))

	// verify that we'll also backfill time (this is weird, but it just feels wrong not to include this)
	// This is ALSO a test that times are clamped upward to 0 (because we're feeding a time here that
	// occurs before the tile's basetime).
	priorBox := TrackedBox{
		Time: base.Add(-1500 * time.Millisecond),
		Box:  nn.MakeRect(5, 6, 13, 8),
	}
	obj.Boxes = append([]TrackedBox{priorBox}, obj.Boxes...)
	b.updateObject(obj)
	require.Equal(t, "1111110000", b.classes[obj.Class].formatRange(0, 10))
	roundtripTile(t, b)
}

// Here we're testing writing and time clamping at the end of a tile
func TestTileBuilder2(t *testing.T) {
	base := tileIdxToTime(12345, 0)
	b := newTileBuilder(0, base, 100)
	obj := &TrackedObject{
		ID:     1,
		Camera: 99,
		Class:  123,
	}
	// sense check that the 2nd last bit (i.e. Second 1022) is toggled
	obj.Boxes = append(obj.Boxes, TrackedBox{
		Time: base.Add(1022 * time.Second),
		Box:  nn.MakeRect(5, 6, 13, 8),
	})
	b.updateObject(obj)
	require.Equal(t, "0000000010", b.classes[obj.Class].formatRange(1014, 1024))

	// test clamping (5000 seconds way exceeds our limit of 1024 seconds per tile)
	obj.Boxes = append(obj.Boxes, TrackedBox{
		Time: base.Add(5000 * time.Second),
		Box:  nn.MakeRect(5, 6, 13, 8),
	})
	b.updateObject(obj)
	require.Equal(t, "0000000011", b.classes[obj.Class].formatRange(1014, 1024))
	roundtripTile(t, b)
}

// Verify that we can operate without any boxes (only using LastSeen)
func TestTileBuilder3(t *testing.T) {
	base := tileIdxToTime(12345, 0)
	b := newTileBuilder(0, base, 100)
	obj := &TrackedObject{
		ID:       1,
		Camera:   99,
		Class:    123,
		LastSeen: base.Add(3 * time.Second),
	}
	b.updateObject(obj)
	require.Equal(t, "0001000000", b.classes[obj.Class].formatRange(0, 10))

	obj.LastSeen = base.Add(4 * time.Second)
	b.updateObject(obj)
	require.Equal(t, "0001100000", b.classes[obj.Class].formatRange(0, 10))

	roundtripTile(t, b)
}

// Verify errors
func TestTileBuilder4(t *testing.T) {
	base := tileIdxToTime(12345, 0)
	b := newTileBuilder(0, base, 1)
	// t0 >= TileWidth
	obj := &TrackedObject{
		ID:       1,
		Camera:   99,
		Class:    123,
		LastSeen: base.Add(1050 * time.Second),
	}
	require.Equal(t, ErrInvalidTimeRange, b.updateObject(obj))

	// too many classes
	obj.Class = 123
	obj.LastSeen = base.Add(3 * time.Second)
	require.NoError(t, b.updateObject(obj))
	obj.Class = 124
	require.Equal(t, ErrTooManyClasses, b.updateObject(obj))
}

// Verify encoding/decoding (compression) of tile bitmap
func TestTileBuilder5(t *testing.T) {
	base := tileIdxToTime(12345, 0)
	A := newTileBuilder(0, base, 100)
	line1, err := A.getBitmapForClass(1)
	require.NoError(t, err)
	line2, err := A.getBitmapForClass(2)
	require.NoError(t, err)
	// line1 is a pathologically bad case for our on/off encoder
	for i := 0; i < TileWidth; i += 2 {
		line1.setBit(uint32(i))
	}
	// line2 compresses well
	line2.setBitRange(0, 10)
	line2.setBitRange(7, 200)
	roundtripTile(t, A)
}

// encode and decode a tilebuilder, and verify that our bitmaps come out the same
func roundtripTile(t *testing.T, tb *tileBuilder) {
	blob := tb.writeBlob()
	B, err := readBlobIntoTileBuilder(tb.tileIdx, 0, blob, 100)
	require.NoError(t, err)
	require.Equal(t, len(tb.classes), len(B.classes))
	for cls, lineA := range tb.classes {
		lineB := B.classes[cls]
		require.Equal(t, lineA.formatRange(0, TileWidth), lineB.formatRange(0, TileWidth))
	}
}

func TestMisc(t *testing.T) {
	// Just checking Go's conversions from float64 to uint32.
	// Short answer: No, it does not clamp before converting.
	m3 := -3.0
	mlarge := 1e15
	fmt.Printf("uint32(%v) = %v, uint32(%v) = %v\n", m3, uint32(m3), mlarge, uint32(mlarge))
}
