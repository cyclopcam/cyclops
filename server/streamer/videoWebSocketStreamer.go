package streamer

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/bluenviron/mediacommon/pkg/codecs/h264"
	"github.com/cyclopcam/cyclops/pkg/log"
	"github.com/cyclopcam/cyclops/pkg/videox"
	"github.com/cyclopcam/cyclops/server/camera"
	"github.com/cyclopcam/cyclops/server/monitor"
	"github.com/gorilla/websocket"
)

type webSocketMsg int

const (
	webSocketMsgPause  webSocketMsg = iota // pause stream (eg browser tab deactivated)
	webSocketMsgResume                     // resume stream (eg browser tab reactivated)
)

// Sent by client over websocket
// SYNC-WEBSOCKET-JSON-MSG
type webSocketJSON struct {
	Command string `json:"command"`
}

// Queued data that must be sent over the websocket
// Either videoFrame or detectionResult will be non-nil
type webSocketSendPacket struct {
	videoFrame *videox.VideoPacket
	detection  *monitor.AnalysisState
}

// When we send a message on the websocket, it's either a BINARY frame, in which case
// it's a video packet. Or it's a TEXT frame, in which case it's this.
// SYNC-CAMERA-WEBSOCKET-STRING-MESSAGE
type webSocketSendStringMessage struct {
	Type      string                 `json:"type"` // Only type of message of "detection"
	Detection *monitor.AnalysisState `json:"detection"`
}

// Number of packets (should be closely correlated with number of frames) that we will buffer
// on the send side, before dropping packets to the sender.
// NOTE: If a camera's IDR interval is greater than this number, then sendBacklog will often fail,
// because we'll run out of buffer space before we've sent an IDR. We should really make this
// buffer size dynamic, and dependent on the IDR interval.
// eg.. perhaps max(20, IDRInterval+1), or something like that.
const WebSocketSendBufferSize = 50

var nextWebSocketStreamerID int64

type VideoWebSocketStreamer struct {
	log        log.Log
	streamerID int64 // Intended to aid in logging/debugging
	incoming   camera.StreamSinkChan
	//trackID         int
	closed            atomic.Bool
	paused            atomic.Bool
	fromWebSocket     chan webSocketMsg
	sendQueue         chan webSocketSendPacket
	detections        chan *monitor.AnalysisState
	lastDropMsg       time.Time
	lastPacketRecvID  int64
	lastPacketMissMsg time.Time
	nPacketsDropped   int64
	nPacketsSent      int64
	lastLogTime       time.Time
	debug             bool
	logPacketCount    bool
}

func RunVideoWebSocketStreamer(cameraName string, logger log.Log, conn *websocket.Conn, stream *camera.Stream, backlog *camera.VideoRingBuffer, detections chan *monitor.AnalysisState) {
	streamerID := atomic.AddInt64(&nextWebSocketStreamerID, 1)

	streamer := &VideoWebSocketStreamer{
		incoming:       make(camera.StreamSinkChan, camera.StreamSinkChanDefaultBufferSize),
		streamerID:     streamerID,
		log:            log.NewPrefixLogger(logger, fmt.Sprintf("Camera %v WebSocket %v", cameraName, streamerID)),
		sendQueue:      make(chan webSocketSendPacket, WebSocketSendBufferSize),
		detections:     detections,
		debug:          false,
		logPacketCount: false, // SYNC-LOG-PACKET-COUNT
	}

	streamer.run(conn, stream, backlog)
}

func (s *VideoWebSocketStreamer) OnConnect(stream *camera.Stream) (camera.StreamSinkChan, error) {
	//s.trackID = stream.H264TrackID
	if s.debug {
		//s.log.Infof("OnConnect trackID:%v", s.trackID)
		s.log.Infof("OnConnect %v", stream.Ident)
	}
	return s.incoming, nil
}

func (s *VideoWebSocketStreamer) onPacketRTP(packet *videox.VideoPacket) {
	if s.debug {
		s.log.Infof("onPacketRTP")
	}

	// Detect if sender is dropping packets
	if s.lastPacketRecvID != 0 && packet.ValidRecvID != s.lastPacketRecvID+1 && time.Now().Sub(s.lastPacketMissMsg) > 3*time.Second {
		s.log.Infof("onPacketRTP packet miss %v -> %v", s.lastPacketRecvID, packet.ValidRecvID)
		s.lastPacketMissMsg = time.Now()
	}
	s.lastPacketRecvID = packet.ValidRecvID

	now := time.Now()
	if len(s.sendQueue) >= WebSocketSendBufferSize {
		s.nPacketsDropped++
		if now.Sub(s.lastDropMsg) > 5*time.Second {
			s.log.Infof("Dropped %v/%v packets", s.nPacketsDropped, s.nPacketsDropped+s.nPacketsSent)
			s.lastDropMsg = now
		}
	} else {
		s.nPacketsSent++
		if s.logPacketCount && s.nPacketsSent%30 == 0 {
			// This log is used in conjunction with a similar console.log in the web client, to debug stale frame/backlog issues.
			// Search for SYNC-LOG-PACKET-COUNT
			s.log.Infof("Sent %v", s.nPacketsSent)
		}
		if now.Sub(s.lastLogTime) > 60*time.Second {
			s.log.Infof("Sent %v/%v packets", s.nPacketsSent, s.nPacketsDropped+s.nPacketsSent)
			s.lastLogTime = now
		}
		s.sendQueue <- webSocketSendPacket{
			videoFrame: packet,
		}
	}
}

func (s *VideoWebSocketStreamer) onDetection(detection *monitor.AnalysisState) {
	// We really don't want to block on a full channel here, because that would cause
	// the NN monitor system to block.
	if len(s.sendQueue) >= WebSocketSendBufferSize*3/4 {
		return
	}
	s.sendQueue <- webSocketSendPacket{
		detection: detection,
	}
}

func (s *VideoWebSocketStreamer) run(conn *websocket.Conn, stream *camera.Stream, backlog *camera.VideoRingBuffer) {
	//s.trackID = stream.H264TrackID

	if s.debug {
		//s.log.Infof("Run start, trackID:%v", s.trackID)
		s.log.Infof("Run start, stream:%v", stream.Ident)
	}

	stream.ConnectSink("WebSocket", s.incoming)
	defer stream.RemoveSink(s.incoming)
	defer conn.Close()

	s.fromWebSocket = make(chan webSocketMsg, 1)
	go s.webSocketReader(conn)
	go s.webSocketWriter(conn)

	if s.debug {
		s.log.Infof("Run ready")
	}

	s.closed.Store(false)
	s.paused.Store(false)
	webSocketClosed := false

	if backlog != nil {
		s.sendBacklog(backlog)
	}

	for !s.closed.Load() {
		select {
		case msg := <-s.incoming:
			switch msg.Type {
			case camera.StreamMsgTypeClose:
				s.log.Infof("Run StreamMsgTypeClose")
				s.closed.Store(true)
			case camera.StreamMsgTypePacket:
				if !s.paused.Load() {
					s.onPacketRTP(msg.Packet)
				}
			}
		case wsMsg, ok := <-s.fromWebSocket:
			if !ok {
				s.log.Infof("Run webSocketMsgClosed")
				webSocketClosed = true
				s.closed.Store(true)
			}
			switch wsMsg {
			case webSocketMsgPause:
				s.paused.Store(true)
			case webSocketMsgResume:
				s.paused.Store(false)
			}
		case detection := <-s.detections:
			if !s.paused.Load() {
				s.onDetection(detection)
			}
		}
	}
	if s.debug {
		s.log.Infof("Run closing")
	}
	//close(s.fromWebSocket)
	close(s.sendQueue)
	if !webSocketClosed {
		// should perhaps use WriteControl(Close) instead of hard closing
		conn.Close()
	}
}

func (s *VideoWebSocketStreamer) sendBacklog(backlog *camera.VideoRingBuffer) {
	s.log.Infof("Sending backlog of frames")
	backlog.BufferLock.Lock()
	defer backlog.BufferLock.Unlock()
	packetIdx := backlog.FindLatestIDRPacketNoLock()
	if packetIdx == -1 {
		return
	}
	top := backlog.Buffer.Len()
	// send all recent packets, starting at the IDR
	for i := packetIdx; i < top; i++ {
		if len(s.sendQueue) >= WebSocketSendBufferSize {
			// just give up before blocking... haven't really thought what the best thing is to do in a case like this
			s.log.Infof("sendBacklog giving up, because sendQueue is full")
			break
		}
		if s.debug {
			s.log.Infof("sendBacklog sending packet %v", i)
		}
		_, packet, _ := backlog.Buffer.Peek(i)
		cloned := packet.Clone()
		cloned.IsBacklog = true
		s.sendQueue <- webSocketSendPacket{
			videoFrame: cloned,
		}
	}

	if s.debug {
		s.log.Infof("sendBacklog done")
	}
}

// Read from the websocket and post to our own channel, so that we can
// run a single loop that handles reads from websocket and reads from camera.
func (s *VideoWebSocketStreamer) webSocketReader(conn *websocket.Conn) {
	for {
		msgType, data, err := conn.ReadMessage()
		if err != nil {
			if s.debug {
				s.log.Infof("webSocketReader conn.ReadMessage error: %v", err)
			}
			break
		}
		//s.log.Infof("received %v message from websocket (len %v) [%v]", msgType, len(data), data[:3])
		if msgType == websocket.TextMessage {
			msg := webSocketJSON{}
			if err := json.Unmarshal(data, &msg); err != nil {
				s.log.Infof("webSocketReader failed to decode JSON: %v", err)
			} else {
				s.log.Infof("Received %v command from websocket", msg.Command)
				// SYNC-WEBSOCKET-COMMANDS
				switch msg.Command {
				case "pause":
					s.fromWebSocket <- webSocketMsgPause
				case "resume":
					s.fromWebSocket <- webSocketMsgResume
				default:
					s.log.Infof("Unknown websocket message from client: '%v'", msg.Command)
				}
			}
		}
	}
	//s.fromWebSocket <- webSocketMsgClosed
	close(s.fromWebSocket)
}

// Run a thread that is responsible for writing to the websocket.
// We run this on a separate thread so that if a client (aka browser) is slow,
// it doesn't end up blocking camera packets from being received,
// and we can detect the blockage.
func (s *VideoWebSocketStreamer) webSocketWriter(conn *websocket.Conn) {
	sentIDR := false
	for {
		pkt, more := <-s.sendQueue
		if !more || s.closed.Load() {
			if s.debug {
				s.log.Infof("webSocketWriter closing. more:%v, s.closed:%v", more, s.closed.Load())
			}
			break
		}

		if s.paused.Load() {
			// When paused, drop all queued frames.
			// This will quickly drain the queue, whereafter we'll stop receiving packets,
			// because the main loop will just drop RTSP packets when paused.
			continue
		}
		if pkt.videoFrame != nil {
			frame := pkt.videoFrame
			if !sentIDR && frame.IsIFrame() {
				// Don't send any IFrames until we've sent a keyframe
				continue
			}
			if frame.HasType(h264.NALUTypeIDR) {
				sentIDR = true
			}

			buf := bytes.Buffer{}
			flags := uint32(0)
			if frame.IsBacklog {
				flags |= 1
			}

			binary.Write(&buf, binary.LittleEndian, flags)
			binary.Write(&buf, binary.LittleEndian, uint32(frame.ValidRecvID))
			for _, n := range frame.H264NALUs {
				//if n.PrefixLen == 0 {
				//	buf.Write([]byte{0, 0, 1})
				//}
				//buf.Write(n.Payload)
				buf.Write(n.AsAnnexB().Payload)
			}
			final := buf.Bytes()
			//s.log.Infof("Sending packet: %v", final[:5])
			if err := conn.WriteMessage(websocket.BinaryMessage, final); err != nil {
				s.log.Infof("Error writing to websocket %v: %v", s.streamerID, err)
			}
		} else {
			out := webSocketSendStringMessage{
				Type:      "detection",
				Detection: pkt.detection,
			}
			j, err := json.Marshal(&out)
			if err != nil {
				s.log.Errorf("Failed to marshal websocket string message: %v", err)
			} else {
				conn.WriteMessage(websocket.TextMessage, j)
			}
		}
	}
}
