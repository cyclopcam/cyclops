package mybits

import (
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func testOnOff(t *testing.T, bits []byte) {
	output := make([]byte, 100)
	encodedLength, err := EncodeOnoff(bits, output)
	require.NoError(t, err)
	//require.Equal(t, expectedLength, encodedLength)

	decoded := make([]byte, len(bits))
	decodedBits, err := DecodeOnoff(output[:encodedLength], decoded)
	require.NoError(t, err)
	require.Equal(t, len(bits)*8, decodedBits)

	// stress the "buffer not large enough" decode path
	decoded = make([]byte, len(bits)-1)
	decodedBits, err = DecodeOnoff(output[:encodedLength], decoded)
	require.Equal(t, ErrOutOfSpace, err)

	if encodedLength > 0 {
		// stress the "buffer not large enough" encode path
		encodedLength, err = EncodeOnoff(bits, output[:encodedLength-1])
		require.Equal(t, ErrOutOfSpace, err)
	}
}

func TestOnOff(t *testing.T) {
	testOnOff(t, []byte{0x00})
	testOnOff(t, []byte{0xff})
	testOnOff(t, []byte{0xff, 0x00})
	testOnOff(t, []byte{0xff, 0x10})
}

/*
Encode1 (pure on/off):

	onoff_test.go:67: Avg encoded length: 22.7
	onoff_test.go:68: Min encoded length: 3
	onoff_test.go:69: Max encoded length: 257

Encode2 (on/off and raw mixed, negative varint for raw):

	onoff_test.go:67: Avg encoded length: 32.4
	onoff_test.go:68: Min encoded length: 3
	onoff_test.go:69: Max encoded length: 187

Encode3 (on/off with 4-bit varints):

	onoff_test.go:86: Avg encoded length: 16.6
	onoff_test.go:87: Min encoded length: 3
	onoff_test.go:88: Max encoded length: 140
*/
func TestPerf(t *testing.T) {
	raw, err := os.ReadFile("dataset_tiles1.txt")
	require.NoError(t, err)
	lengths := []int{}
	total := 0
	minLen := 1000
	maxLen := 0
	nlines := 0
	for _, line := range strings.Split(string(raw), "\n") {
		if len(line) == 0 {
			continue
		}
		raw, err := hex.DecodeString(strings.TrimSpace(line))
		require.NoError(t, err)
		require.Equal(t, 128, len(raw))
		enc := make([]byte, 800)
		length, err := EncodeOnoff(raw, enc)
		require.NoError(t, err)
		//lengthB, err := EncodeOnoff3(raw, enc)
		//if lengthB > length {
		//	t.Logf("Line %v: %v vs %v", iline, length, lengthB)
		//	printBits(t, raw)
		//	break
		//}

		// Verify correctness
		raw2 := make([]byte, len(raw))
		decodedLength, err := DecodeOnoff(enc[:length], raw2)
		require.NoError(t, err)
		require.Equal(t, len(raw)*8, decodedLength)

		// Collect stats
		lengths = append(lengths, length)
		total += length
		minLen = min(minLen, length)
		maxLen = max(maxLen, length)
		nlines++
	}
	t.Logf("Avg encoded length: %.1f", float64(total)/float64(nlines))
	t.Logf("Min encoded length: %v", minLen)
	t.Logf("Max encoded length: %v", maxLen)
}

func printBits(t *testing.T, b []byte) {
	for i := 0; i < len(b); i += 16 {
		piece := b[i:min(i+16, len(b))]
		str := ""
		for _, u8 := range piece {
			str += fmt.Sprintf("%08b ", u8)
		}
		t.Logf("%v", str)
	}
}

var bruteCheck map[uint8]bool

func printByte(v uint8) {
	check := v == 0 || v == 0xff || (v&(v+1)) == 0
	fmt.Printf("%3v %08b %v %v\n", v, v, bruteCheck[v], check)
}

func isContiguous(v uint8) bool {
	//return v == 0 || v == 0xff || (v&(v+1)) == 0 || (^v&(^v+1)) == 0
	return (v&(v+1)) == 0 || (^v&(^v+1)) == 0
}

func TestContiguousBits(t *testing.T) {
	// There are 16 different patterns of contiguous on/off bits in a byte,
	// and we're trying to figure out a way to compute whether this is true.
	for i := 0; i < 8; i++ {
		printByte(uint8(0xff) >> i)
	}
	for i := 0; i < 8; i++ {
		printByte(^(uint8(0xff) >> i))
	}
	for i := 0; i < 256; i++ {
		v := uint8(i)
		if isContiguous(v) != bruteCheck[v] {
			t.Fatalf("Mismatch at %v %08b (should be %v)", v, v, bruteCheck[v])
		}
	}
}

// I just created this function to make a little test sample for validating the WASM port
func TestMakeTestPattern(t *testing.T) {
	input := []byte{3, 255, 255, 255, 255, 255, 7, 127}
	output := make([]byte, 100)
	outputSize, err := EncodeOnoff(input, output)
	require.NoError(t, err)
	fmt.Printf("Input bytes: %v\n", input)
	fmt.Printf("Output bytes: %v\n", output[:outputSize])
}

func init() {
	bruteCheck = map[uint8]bool{
		0b11111111: true,
		0b01111111: true,
		0b00111111: true,
		0b00011111: true,
		0b00001111: true,
		0b00000111: true,
		0b00000011: true,
		0b00000001: true,
		0b00000000: true,
		0b10000000: true,
		0b11000000: true,
		0b11100000: true,
		0b11110000: true,
		0b11111000: true,
		0b11111100: true,
		0b11111110: true,
	}
}
