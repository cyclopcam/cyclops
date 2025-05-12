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
		PTS:     0,
	}

	for _, p := range input {
		if p.PTS != packet.WallPTS {
			pb.Packets = append(pb.Packets, packet)
			packet = &VideoPacket{
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

	return &pb
}
