package rle

import (
	"bytes"
	"testing"
)

func testRLERoundTrip(t *testing.T, data []byte) {
	compressed := Compress(data)
	decompressed := make([]byte, len(data))
	decompressedSize, err := Decompress(compressed, decompressed)
	if err != nil {
		t.Errorf("Decompression failed: %v", err)
	}
	if decompressedSize != len(data) {
		t.Errorf("Decompressed size %d does not match original size %d", decompressedSize, len(data))
	}
	if !bytes.Equal(data, decompressed) {
		t.Errorf("Decompressed data does not match original data")
	}

	if len(data) > 0 {
		decompressedSize, err = Decompress(compressed, decompressed[:len(decompressed)-1])
		if err != ErrNotEnoughSpace {
			t.Errorf("Expected ErrNotEnoughSpace, got %v", err)
		}
	}
}

func TestRLE(t *testing.T) {
	testRLERoundTrip(t, []byte{})
	testRLERoundTrip(t, []byte{1})
	testRLERoundTrip(t, []byte{1, 1, 1})
	testRLERoundTrip(t, []byte{1, 2, 4, 4, 4, 4, 1, 1, 9, 9, 9})
}
