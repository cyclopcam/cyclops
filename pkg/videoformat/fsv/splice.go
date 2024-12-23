package fsv

import (
	"slices"
)

// The functions in this file deal with the somewhat rare, but not that rare, condition where
// we attempt to write packets that have already been written.
// Why would this happen?
// Here is a common scenario:
// 1. Recording mode is "object detection"
// 2. An object is detected, and we record for 15 seconds.
// 3. We stop recording.
// 4. Five seconds later, an object is detected again.
// 5. When we detect an object, we don't just start recording from that moment.
//    We include 15 seconds of history that preceded the object detection moment.
// These 15 seconds will overlap with the recording from the previous detection.
//
// We could push this requirement out onto the VideoRecorder object, but I like the
// robustness of having it inside here, and the fact that the VideoRecorder never
// needs to think about this little detail.

// I've been getting errors like this:
// 2024-12-22 05:27:47.558271 Warning Archive: Splice found packet at matching time (2024-12-22 07:27:44.467281433 +0200 SAST m=+4562.617532283),
//                            but lengths are different (old 304697, new 27)
// SPS/PPS packets?
// Yes, this makes perfect sense. When SPS/PPS packets come in, they are bundled together in 2 or 3 NALUs, all with an identical PTS.
// So we make provision for this.
// The change I made here was to insist that PTS and Payload length match between the existing and incoming packets.

// Returns only the NALUs in 'packets' that are after the last packet in 'recent'.
// However, if the two sets of NALUs do not overlap, then we scan through 'packets', and return
// the array from the first keyframe onwards.
func (a *Archive) splicePacketsBeforeWrite(stream *videoStream, track string, packets []NALU) []NALU {
	recent := stream.recentWrite[track]
	if len(recent) == 0 || len(packets) == 0 {
		return packets
	}

	last := recent[len(recent)-1]
	if last.PTS == packets[0].PTS || last.PTS.Before(packets[0].PTS) {
		return packets
	}

	// Find the last packet from 'recent' inside 'packets'
	for i := range packets {
		if packets[i].PTS == last.PTS && len(packets[i].Payload) == int(last.Length) {
			//a.debugPacketSplice("Splice found packet at matching time (%v), i = %v", packets[i].PTS, i)
			//if len(packets[i].Payload) != int(last.Length) {
			//	// Not sure what else we can do in this scenario. I guess we'll get some garbled video.
			//	// I don't expect this in practice, but I leave it here as a sanity check.
			//	a.log.Warnf("Splice found packet at matching time (%v), but lengths are different (old %v, new %v)", packets[i].PTS, last.Length, len(packets[i].Payload))
			//}
			return packets[i+1:]
		} else if packets[i].PTS.After(last.PTS) {
			// We didn't find an exact match, but this packet is at least AFTER the last packet in 'recent'.
			// So we find the next keyframe in packets, and return everything from that point onwards.
			for j := i; j < len(packets); j++ {
				if packets[j].IsKeyFrame() || packets[j].IsEssentialMeta() {
					a.debugPacketSplice("Splice found next keyframe at i = %v", j)
					return packets[j:]
				}
			}
			a.debugPacketSplice("Splice found no keyframe after last packet in recent")
			return nil
		}
	}
	return packets
}

func (a *Archive) addPacketsToRecentWriteList(stream *videoStream, track string, packets []NALU) {
	// Clone NALUs, then set their payload to nil, so that the garbage collector can reclaim the payload memory.
	packets = slices.Clone(packets)
	for i := range packets {
		packets[i].Length = int32(len(packets[i].Payload))
		packets[i].Payload = nil
	}

	recent := stream.recentWrite[track]
	if len(recent) > a.recentWriteMaxQueue {
		// discard older packets
		recent = recent[len(recent)-a.recentWriteMaxQueue:]
	}
	recent = append(recent, packets...)

	stream.recentWrite[track] = recent
}

func (a *Archive) debugPacketSplice(m string, args ...any) {
	a.log.Infof("Packet splice: "+m, args...)
}
