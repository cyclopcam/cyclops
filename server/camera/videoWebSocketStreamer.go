package camera

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/aler9/gortsplib/pkg/h264"
	"github.com/bmharper/cyclops/server/log"
	"github.com/bmharper/cyclops/server/videox"
	"github.com/gorilla/websocket"
)

type webSocketMsg int

const (
	webSocketMsgClosed webSocketMsg = iota // The websocket client has closed the channel
	webSocketMsgPause                      // pause stream (eg browser tab deactivated)
	webSocketMsgResume                     // resume stream (eg browser tab reactivated)
)

// Sent by client over websocket
// SYNC-WEBSOCKET-JSON-MSG
type webSocketJSON struct {
	Command string `json:"command"`
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
	log             log.Log
	streamerID      int64 // Intended to aid in logging/debugging
	incoming        StreamSinkChan
	trackID         int
	closed          atomic.Bool
	paused          atomic.Bool
	fromWebSocket   chan webSocketMsg
	sendQueue       chan *videox.DecodedPacket
	lastDropMsg     time.Time
	nPacketsDropped int64
	nPacketsSent    int64
	lastLogTime     time.Time
	debug           bool
	logPacketCount  bool // SYNC-LOG-PACKET-COUNT
}

func NewVideoWebSocketStreamer(cameraName string, logger log.Log) *VideoWebSocketStreamer {
	streamerID := atomic.AddInt64(&nextWebSocketStreamerID, 1)
	return &VideoWebSocketStreamer{
		incoming:   make(StreamSinkChan, StreamSinkChanDefaultBufferSize),
		streamerID: streamerID,
		log:        log.NewPrefixLogger(logger, fmt.Sprintf("Camera %v WebSocket %v", cameraName, streamerID)),
		sendQueue:  make(chan *videox.DecodedPacket, WebSocketSendBufferSize),
		debug:      false,
	}
}

func (s *VideoWebSocketStreamer) OnConnect(stream *Stream) (StreamSinkChan, error) {
	s.trackID = stream.H264TrackID
	if s.debug {
		s.log.Infof("OnConnect trackID:%v", s.trackID)
	}
	return s.incoming, nil
}

func (s *VideoWebSocketStreamer) onPacketRTP(packet *videox.DecodedPacket) {
	if s.debug {
		s.log.Infof("onPacketRTP")
	}

	now := time.Now()
	if len(s.sendQueue) >= WebSocketSendBufferSize {
		s.nPacketsDropped++
		if now.Sub(s.lastDropMsg) > 5*time.Second {
			s.log.Infof("Dropped %v/%v packets", s.nPacketsDropped, s.nPacketsDropped+s.nPacketsSent)
			s.lastDropMsg = now
		}
	} else {
		s.nPacketsSent++
		if s.logPacketCount && s.nPacketsSent%60 == 0 {
			// This log is used in conjunction with a similar console.log in the web client, to debug stale frame/backlog issues.
			// Search for SYNC-LOG-PACKET-COUNT
			s.log.Infof("Sent %v", s.nPacketsSent)
		}
		if now.Sub(s.lastLogTime) > 60*time.Second {
			s.log.Infof("Sent %v/%v packets", s.nPacketsSent, s.nPacketsDropped+s.nPacketsSent)
			s.lastLogTime = now
		}
		s.sendQueue <- packet
	}
}

func (s *VideoWebSocketStreamer) Run(conn *websocket.Conn, stream *Stream, backlog *VideoDumpReader) {
	if s.debug {
		s.log.Infof("Run start")
	}

	stream.ConnectSink(s, false)
	defer stream.RemoveSink(s)
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
			case StreamMsgTypeClose:
				s.log.Infof("Run StreamMsgTypeClose")
				s.closed.Store(true)
			case StreamMsgTypePacket:
				if !s.paused.Load() {
					s.onPacketRTP(msg.Packet)
				}
			}
		case wsMsg := <-s.fromWebSocket:
			switch wsMsg {
			case webSocketMsgClosed:
				s.log.Infof("Run webSocketMsgClosed")
				webSocketClosed = true
				s.closed.Store(true)
			case webSocketMsgPause:
				s.paused.Store(true)
			case webSocketMsgResume:
				s.paused.Store(false)
			}
		}
	}
	if s.debug {
		s.log.Infof("Run closing")
	}
	close(s.fromWebSocket)
	close(s.sendQueue)
	if !webSocketClosed {
		// should perhaps use WriteControl(Close) instead of hard closing
		conn.Close()
	}
}

func (s *VideoWebSocketStreamer) sendBacklog(backlog *VideoDumpReader) {
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
		s.sendQueue <- cloned
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
	s.fromWebSocket <- webSocketMsgClosed
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
		if !sentIDR && pkt.IsIFrame() {
			// Don't send any IFrames until we've sent a keyframe
			continue
		}
		if pkt.HasType(h264.NALUTypeIDR) {
			sentIDR = true
		}

		buf := bytes.Buffer{}
		//pts := float64(pkt.H264PTS.Microseconds())
		//binary.Write(&buf, binary.LittleEndian, pts)
		//foo1 := uint32(123)
		//foo2 := uint32(456)
		//binary.Write(&buf, binary.LittleEndian, foo1)
		//binary.Write(&buf, binary.LittleEndian, foo2)
		flags := uint32(0)
		if pkt.IsBacklog {
			flags |= 1
		}

		binary.Write(&buf, binary.LittleEndian, flags)
		for _, n := range pkt.H264NALUs {
			if n.PrefixLen == 0 {
				buf.Write([]byte{0, 0, 1})
			}
			buf.Write(n.Payload)
		}
		final := buf.Bytes()
		//s.log.Infof("Sending packet: %v", final[:5])
		if err := conn.WriteMessage(websocket.BinaryMessage, final); err != nil {
			s.log.Infof("Error writing to websocket %v: %v", err)
		}
	}
}
