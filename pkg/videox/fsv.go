package videox

import (
	"github.com/cyclopcam/cyclops/pkg/videoformat/fsv"
)

// Convert FSV packets to our VideoPacket format
func ExtractFsvPackets(input []fsv.NALU) *PacketBuffer {
	pb := PacketBuffer{}
	if len(input) == 0 {
		return &pb
	}

	packet := &VideoPacket{
		WallPTS: input[0].PTS,
	}

	for _, p := range input {
		if p.PTS != packet.WallPTS {
			pb.Packets = append(pb.Packets, packet)
			packet = &VideoPacket{
				WallPTS: p.PTS,
			}
		}
		n := NALU{
			PayloadIsAnnexB: p.Flags&fsv.NALUFlagAnnexB != 0,
			Payload:         p.Payload,
		}
		packet.H264NALUs = append(packet.H264NALUs, n)
	}
	pb.Packets = append(pb.Packets, packet)

	return &pb
}
