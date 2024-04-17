package fsv

import (
	"slices"

	"github.com/cyclopcam/cyclops/pkg/videoformat/rf1"
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

func (a *Archive) splicePacketsBeforeWrite(stream *videoStream, track string, packets []rf1.NALU) []rf1.NALU {
	if len(packets) == 0 {
		return packets
	}
	recent := stream.recentWrite[track]
	if len(recent) == 0 {
		return packets
	}
	if recent[0].PTS.After(packets[0].PTS) {
		return packets
	}
	// Find the last packet from 'recent' inside 'packets'
	last := recent[len(recent)-1]
	for i := range packets {
		if packets[i].PTS == last.PTS {
			//a.debugPacketSplice("Splice found packet at matching time (%v), i = %v", packets[i].PTS, i)
			if len(packets[i].Payload) != int(last.Length) {
				// Not sure what else we can do in this scenario. I guess we'll get some garbled video.
				// I don't expect this in practice, but I leave it here as a sanity check.
				a.log.Warnf("Splice found packet at matching time (%v), but lengths are different (old %v, new %v)", packets[i].PTS, last.Length, len(packets[i].Payload))
			}
			return packets[i+1:]
		}
	}
	return packets
}

func (a *Archive) addPacketsToRecentWriteList(stream *videoStream, track string, packets []rf1.NALU) {
	// Clone NALUs, then set their payload to nil, so that the garbage collector can reclaim the payload memory.
	packets = slices.Clone(packets)
	for i := range packets {
		packets[i].Length = int64(len(packets[i].Payload))
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
