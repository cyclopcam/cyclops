package videodb

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cyclopcam/cyclops/pkg/log"
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
	require.ErrorIs(t, ErrInvalidTimeRange, b.updateObject(obj))

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

func TestMulticlassTile(t *testing.T) {
	b1 := createTestTile(0, 128, map[uint32]string{
		2: "0-9",
		7: "2-12",
		8: "5-7",
	})
	blob := b1.writeBlob()
	b2, err := readBlobIntoTileBuilder(128, 0, blob, 100)
	require.NoError(t, err)
	verifyTileBits(t, b2, 0, 128, map[uint32]string{
		2: "0-9",
		7: "2-12",
		8: "5-7",
	})
}

// 0,1,4-7 -> 11001111
func rangeStringToBits(s string, bmp *bitmapLine) {
	for _, part := range strings.Split(s, ",") {
		if strings.Contains(part, "-") {
			start, end, _ := strings.Cut(part, "-")
			istart, _ := strconv.Atoi(start)
			iend, _ := strconv.Atoi(end)
			bmp.setBitRange(uint32(istart), uint32(iend))
		} else {
			i, _ := strconv.Atoi(part)
			bmp.setBit(uint32(i))
		}
	}
}

func createTestTile(level, tileIdx uint32, classToRangeString map[uint32]string) *tileBuilder {
	tb := newTileBuilder(level, tileIdxToTime(tileIdx, level), 100)
	for cls, bits := range classToRangeString {
		bmp, _ := tb.getBitmapForClass(cls)
		rangeStringToBits(bits, bmp)
	}
	return tb
}

func insertTestTile(t *testing.T, vdb *VideoDB, camera, level, tileIdx uint32, classToRangeString map[uint32]string) *tileBuilder {
	tb := createTestTile(level, tileIdx, classToRangeString)
	tile := EventTile{
		Level:  level,
		Camera: camera,
		Start:  tileIdx,
		Tile:   tb.writeBlob(),
	}
	err := vdb.db.Create(&tile).Error
	require.NoError(t, err)
	return tb
}

func verifyTileBits(t *testing.T, tb *tileBuilder, level, tileIdx uint32, classToRangeString map[uint32]string) {
	for cls, rangeString := range classToRangeString {
		actualBits, err := tb.getBitmapForClass(cls)
		require.NoError(t, err)
		expectedBits := bitmapLine{}
		rangeStringToBits(rangeString, &expectedBits)
		// Show a short diff first, because this is more readable, and most of our
		// errors show up here.
		require.Equal(t, expectedBits.formatRange(0, 80), actualBits.formatRange(0, 80), "First half, level %v, tile %v, class %v", level, tileIdx, cls)
		require.Equal(t, expectedBits.formatRange(512, 600), actualBits.formatRange(512, 600), "Second half, level %v, tile %v, class %v", level, tileIdx, cls)
		// Compare the full range
		expectLong := expectedBits.formatRange(0, TileWidth)
		actualLong := actualBits.formatRange(0, TileWidth)
		if expectLong != actualLong {
			require.Fail(t, "Mismatch", expectLong, actualLong)
		}
	}
}

func verifyTileBitsInDB(t *testing.T, vdb *VideoDB, camera, level, tileIdx uint32, classToRangeString map[uint32]string) {
	tile := EventTile{}
	err := vdb.db.First(&tile, "camera = ? AND level = ? AND start = ?", camera, level, tileIdx).Error
	require.NoError(t, err)
	tb, err := readBlobIntoTileBuilder(tileIdx, level, tile.Tile, 100)
	require.NoError(t, err)
	verifyTileBits(t, tb, level, tileIdx, classToRangeString)
}

func TestLevels(t *testing.T) {
	root := "temptest"
	os.RemoveAll(root)
	vdb, err := NewVideoDB(log.NewTestingLog(t), root)
	vdb.debugTileLevelBuild = true
	vdb.maxTileLevel = 5
	require.NoError(t, err)
	tiles := make([]*tileBuilder, 1000)
	tiles[128] = insertTestTile(t, vdb, 1, 0, 128, map[uint32]string{
		2: "0-9",
		7: "2-12",
	})
	tiles[129] = insertTestTile(t, vdb, 1, 0, 129, map[uint32]string{
		2: "0-19",
	})
	tiles[130] = insertTestTile(t, vdb, 1, 0, 130, map[uint32]string{
		2: "0-30",
	})
	tiles[131] = insertTestTile(t, vdb, 1, 0, 131, map[uint32]string{
		2: "0-40",
	})

	bmp1, _ := tiles[128].getBitmapForClass(7)
	require.Equal(t, "00111111111100", bmp1.formatRange(0, 14))

	validateLevel0 := func() {
		// sense check of level 0 tiles
		verifyTileBitsInDB(t, vdb, 1, 0, 128, map[uint32]string{
			2: "0-9",
			7: "2-12",
		})
		verifyTileBitsInDB(t, vdb, 1, 0, 129, map[uint32]string{
			2: "0-19",
		})
		verifyTileBitsInDB(t, vdb, 1, 0, 130, map[uint32]string{
			2: "0-30",
		})
		verifyTileBitsInDB(t, vdb, 1, 0, 131, map[uint32]string{
			2: "0-40",
		})
	}
	validateLevel0()

	// Simulate the passing of time, as we reach the closeout of each tile
	for tileIdx := uint32(128); tileIdx <= uint32(131); tileIdx++ {
		cutoff := tileIdxToTime(tileIdx+1, 0).Add(tileWriterFlushThreshold)
		//t.Logf("TileEndTime: %v, Cutoff: %v", endOfTile(tileIdx, 0), cutoff)
		vdb.buildHigherTiles(map[uint32]uint32{1: tileIdx}, cutoff)
	}

	validateHigherLevels := func() {
		verifyTileBitsInDB(t, vdb, 1, 1, 64, map[uint32]string{
			2: "0-5,512-522",
			7: "1-6",
		})
		verifyTileBitsInDB(t, vdb, 1, 1, 65, map[uint32]string{
			2: "0-15,512-532",
		})
		verifyTileBitsInDB(t, vdb, 1, 2, 32, map[uint32]string{
			2: "0-3,256-261,512-520,768-778",
			7: "0-3",
		})
		// and by the powers of induction... we know the rest of the levels will work ;)
	}
	validateHigherLevels()

	// Phase 2, where we test fillMissingTiles()
	// But first we need to get rid of the higher level tiles.
	vdb.db.Exec("DELETE FROM event_tile WHERE level > 0")
	vdb.setKV("lastTileIdx", 127)
	// Before implementing scan limits in buildHigherTilesForCamera(), we had to use an artifical time
	// here. But after implementing the limits, we can go all the way from 1970 to present without
	// any performance hit.
	//vdb.fillMissingTiles(endOfTile(132, 0))
	vdb.fillMissingTiles(time.Now())
	validateLevel0()
	validateHigherLevels()

	// repeat
	vdb.fillMissingTiles(time.Now())
	validateLevel0()
	validateHigherLevels()
}
