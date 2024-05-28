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

// Initial POC code for on/off encoding
/*
func TestCompare(t *testing.T) {
	// Raw blobs from sqlite DB, RLE encoded.
	//   0      1       1676144  13            010102DC0001FC83FF01039F00
	//   0      1       1676146  11            010102ED0001F891FF013F
	//   0      3       1676146  23            010104EF0005E03F00F801830002807F830004C03FFCFF
	//   0      1       1676147  145           010202060580FF7FFEEF82FF0503C0FF07F882FF01FB88FF01C084FF051FF0F3FFC082FF0207F082FF0401C00F8085FF031FFC078600043FE0FF7F820004F0E3FFF882FF01F182FF04F10F008082FF0C3FFCFBFFC01FF0FFC30F00C082FF041F00807F820005FFC00F008083FF02E3FF840002801F8700077EC01F00E0FF0F940002C07FA00003FEE007B90002F0038C00
	//   0      3       1676147  84            01040405070A03FFF903AB00017E840005E0070080FF940002FC07880002807FA8008C00013F870002FC03950002F801830002FF1F8600017F9F000380FF0F8D0003FCFD019500A40002E007DA00D900017EA600
	//   0      8       1676147  9             010109C600013FB900
	//   0      1       1676148  131           0102020603FCFFF182FF090F00C0FF0100FEFF0783000AFEFF3F00FC07F0FFF87F820006FCE03F80FF3F840005C00FFEFF1F820003F8FF038A00017E820003FEE307850009FFFEFF0700FC07FE0382FF0603F00F00FFF191FF018F85FF018182FF01E182FF05FEFD07F07F82FF820003F00380D10002FC078200013F940002F8039400
	//   0      3       1676148  70            010505040B070AE00002E00F9A0002F0FF8200EC0001FC9300830004F00FF801C00002F0078B0002801F840002F00FA400970004E03FFE03DF0002F8038400DA0002E007A400
	//   0      3       1676149  14            0101058F0001FC850002803FE900
	//   0      1       1676149  32            01010202FF03870004FE03FF0F820001E082FF0600E01F00E03F8200017FE500

	// With header section removed - only the on/off bits remaining.
	testHex := []string{
		"DC0001FC83FF01039F00",
		"ED0001F891FF013F",
		"EF0005E03F00F801830002807F830004C03FFCFF",
		"0580FF7FFEEF82FF0503C0FF07F882FF01FB88FF01C084FF051FF0F3FFC082FF0207F082FF0401C00F8085FF031FFC078600043FE0FF7F820004F0E3FFF882FF01F182FF04F10F008082FF0C3FFCFBFFC01FF0FFC30F00C082FF041F00807F820005FFC00F008083FF02E3FF840002801F8700077EC01F00E0FF0F940002C07FA00003FEE007B90002F0038C00",
		"03FFF903AB00017E840005E0070080FF940002FC07880002807FA8008C00013F870002FC03950002F801830002FF1F8600017F9F000380FF0F8D0003FCFD019500A40002E007DA00D900017EA600",
		"C600013FB900",
		"03FCFFF182FF090F00C0FF0100FEFF0783000AFEFF3F00FC07F0FFF87F820006FCE03F80FF3F840005C00FFEFF1F820003F8FF038A00017E820003FEE307850009FFFEFF0700FC07FE0382FF0603F00F00FFF191FF018F85FF018182FF01E182FF05FEFD07F07F82FF820003F00380D10002FC078200013F940002F8039400",
		"E00002E00F9A0002F0FF8200EC0001FC9300830004F00FF801C00002F0078B0002801F840002F00FA400970004E03FFE03DF0002F8038400DA0002E007A400",
		"8F0001FC850002803FE900",
		"02FF03870004FE03FF0F820001E082FF0600E01F00E03F8200017FE500",
	}
	for _, h := range testHex {
		rleEnc, err := hex.DecodeString(h)
		require.NoError(t, err)
		bits := make([]byte, 1024)
		length, err := Decompress(rleEnc, bits)
		require.NoError(t, err)
		bits = bits[:length]
		//t.Logf("Decompressed %d bytes", length)
		require.Equal(t, 0, length%128, "Decompressed length is not a multiple of 128")
		onoff := encodeOnOff(bits)
		bits2 := decodeOnOff(onoff)
		require.Equal(t, bits, bits2)
		t.Logf("raw %4v,  rle %4v,  onoff %3v", len(bits), len(rleEnc), len(onoff))
	}
}

func encodeOnOff(bits []byte) []byte {
	encoded := make([]byte, 0, len(bits)/20)
	state := getBit(bits, 0)
	start := 0
	i := 0
	if state {
		encoded = append(encoded, 1)
	} else {
		encoded = append(encoded, 0)
	}
	for ; i < len(bits)*8; i++ {
		on := getBit(bits, i)
		if on != state {
			length := i - start
			encoded = binary.AppendVarint(encoded, int64(length))
			start = i
			state = on
		}
	}
	length := i - start
	encoded = binary.AppendVarint(encoded, int64(length))
	return encoded
}

func decodeOnOff(onoff []byte) []byte {
	bits := make([]byte, 1024)
	i := 0
	on := onoff[0] == 1
	onoff = onoff[1:]
	for len(onoff) > 0 {
		length, n := binary.Varint(onoff)
		onoff = onoff[n:]
		if on {
			for j := 0; j < int(length); j++ {
				if i >= len(bits)*8 {
					break
				}
				setBit(bits, i)
				i++
			}
		} else {
			i += int(length)
		}
		on = !on
	}
	return bits[:i/8]
}

func getBit(bits []byte, i int) bool {
	return bits[i/8]&(1<<(i%8)) != 0
}

func setBit(bits []byte, i int) {
	bits[i/8] |= 1 << (i % 8)
}
*/
