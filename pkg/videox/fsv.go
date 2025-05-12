package videox

import (
	"fmt"

	"github.com/cyclopcam/cyclops/pkg/videoformat/fsv"
	"github.com/cyclopcam/cyclops/pkg/videoformat/rf1"
)

func CodecToFsv(codec Codec) string {
	switch codec {
	case CodecH264:
		return rf1.CodecH264
	case CodecH265:
		return rf1.CodecH265
	default:
		panic("Invalid codec")
	}
}

func ParseFsvCodec(codec string) (Codec, error) {
	switch codec {
	case rf1.CodecH264:
		return CodecH264, nil
	case rf1.CodecH265:
		return CodecH265, nil
	default:
		return CodecUnknown, fmt.Errorf("Unknown codec: %v", codec)
	}
}

// Convert FSV packets to our VideoPacket format
func ExtractFsvPackets(fsvCodec string, input []fsv.NALU) (*PacketBuffer, error) {
	codec, err := ParseFsvCodec(fsvCodec)
	if err != nil {
		return nil, err
	}

	pb := PacketBuffer{}
	if len(input) == 0 {
		return &pb, nil
	}

	packet := &VideoPacket{
		Codec:   codec,
		WallPTS: input[0].PTS,
		PTS:     0,
	}

	for _, p := range input {
		if p.PTS != packet.WallPTS {
			pb.Packets = append(pb.Packets, packet)
			packet = &VideoPacket{
				Codec:   codec,
				WallPTS: p.PTS,
				PTS:     p.PTS.Sub(input[0].PTS),
			}
		}
		n := NALU{
			PayloadIsAnnexB: p.Flags&fsv.NALUFlagAnnexB != 0,
			Payload:         p.Payload,
		}
		packet.NALUs = append(packet.NALUs, n)
	}
	pb.Packets = append(pb.Packets, packet)

	return &pb, nil
}
