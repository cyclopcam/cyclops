package camera

import (
	"bytes"
	"time"

	"github.com/aler9/gortsplib"
	"github.com/aler9/gortsplib/pkg/h264"
	"github.com/bmharper/cyclops/server/log"
	"github.com/bmharper/cyclops/server/videox"
	"github.com/gorilla/websocket"
)

type webSocketMsg int

const (
	webSocketMsgClosed webSocketMsg = iota // The websocket client has closed the channel
)

// Number of packets (should be closely correlated with number of frames) that we will buffer
// on the send side, before dropping packets to the sender.
const WebSocketSendBufferSize = 15

type VideoWebSocketStreamer struct {
	log             log.Log
	incoming        StreamSinkChan
	trackID         int
	closed          bool
	fromWebSocket   chan webSocketMsg
	sendQueue       chan *videox.DecodedPacket
	lastDropMsg     time.Time
	nPacketsDropped int64
	nPacketsSent    int64
	lastLogTime     time.Time
	debug           bool
}

func NewVideoWebSocketStreamer(log log.Log) *VideoWebSocketStreamer {
	return &VideoWebSocketStreamer{
		incoming:  make(StreamSinkChan, StreamSinkChanDefaultBufferSize),
		log:       log,
		sendQueue: make(chan *videox.DecodedPacket, WebSocketSendBufferSize),
		debug:     false,
	}
}

func (s *VideoWebSocketStreamer) OnConnect(stream *Stream) (StreamSinkChan, error) {
	s.trackID = stream.H264TrackID
	if s.debug {
		s.log.Infof("VideoWebSocketStreamer.OnConnect trackID:%v", s.trackID)
	}
	return s.incoming, nil
}

func (s *VideoWebSocketStreamer) onPacketRTP(ctx *gortsplib.ClientOnPacketRTPCtx) {
	if ctx.TrackID != s.trackID || ctx.H264NALUs == nil {
		return
	}

	if s.debug {
		s.log.Infof("VideoWebSocketStreamer.onPacketRTP")
	}

	now := time.Now()
	if len(s.sendQueue) >= WebSocketSendBufferSize {
		s.nPacketsDropped++
		if now.Sub(s.lastDropMsg) > 5*time.Second {
			s.log.Infof("Dropped %v/%v packets to websocket connection", s.nPacketsDropped, s.nPacketsDropped+s.nPacketsSent)
			s.lastDropMsg = now
		}
	} else {
		s.nPacketsSent++
		if now.Sub(s.lastLogTime) > 10*time.Second {
			s.log.Infof("Sent %v/%v packets to websocket connection", s.nPacketsSent, s.nPacketsDropped+s.nPacketsSent)
			s.lastLogTime = now
		}
		s.sendQueue <- videox.ClonePacket(ctx)
	}
}

func (s *VideoWebSocketStreamer) Run(conn *websocket.Conn, stream *Stream) {
	if s.debug {
		s.log.Infof("VideoWebSocketStreamer.Run start")
	}

	stream.ConnectSink(s, false)
	defer stream.RemoveSink(s)
	defer conn.Close()

	s.fromWebSocket = make(chan webSocketMsg, 1)
	go s.webSocketReader(conn)
	go s.webSocketWriter(conn)

	if s.debug {
		s.log.Infof("VideoWebSocketStreamer.Run ready")
	}

	s.closed = false
	webSocketClosed := false
	for !s.closed {
		select {
		case msg := <-s.incoming:
			switch msg.Type {
			case StreamMsgTypeClose:
				if s.debug {
					s.log.Infof("VideoWebSocketStreamer.Run StreamMsgTypeClose")
				}
				s.closed = true
				break
			case StreamMsgTypePacket:
				s.onPacketRTP(msg.Packet)
			}
		case wsMsg := <-s.fromWebSocket:
			switch wsMsg {
			case webSocketMsgClosed:
				if s.debug {
					s.log.Infof("VideoWebSocketStreamer.Run webSocketMsgClosed")
				}
				webSocketClosed = true
				s.closed = true
				break
			}
		}
	}
	close(s.fromWebSocket)
	close(s.sendQueue)
	if !webSocketClosed {
		// should perhaps use WriteControl(Close) instead of hard closing
		conn.Close()
	}
}

// Read from the websocket and post to our own channel, so that we can
// run a single loop that handles reads from websocket and reads from camera.
func (s *VideoWebSocketStreamer) webSocketReader(conn *websocket.Conn) {
	for {
		msgType, data, err := conn.ReadMessage()
		if err != nil {
			if s.debug {
				s.log.Infof("VideoWebSocketStreamer.webSocketReader conn.ReadMessage error: %v", err)
			}
			break
		}
		s.log.Infof("VideoWebSocketStreamer received %v message from websocket (len %v) [%v]", msgType, len(data), data[:3])
	}
	s.fromWebSocket <- webSocketMsgClosed
}

// Run a thread that is responsible for writing to the websocket.
// We run this on a separate thread so that if a client is slow,
// it doesn't end up blocking camera packets from being received,
// and we can detect the blockage.
func (s *VideoWebSocketStreamer) webSocketWriter(conn *websocket.Conn) {
	sentIDR := false
	for {
		pkt, more := <-s.sendQueue
		if !more || s.closed {
			if s.debug {
				s.log.Infof("VideoWebSocketStreamer.webSocketWriter closing. more:%v, s.closed:%v", more, s.closed)
			}
			break
		}

		if !sentIDR && pkt.IsIFrame() {
			// Don't send any IFrames until we've sent a keyframe
			continue
		}
		if pkt.HasType(h264.NALUTypeIDR) {
			sentIDR = true
		}

		buf := bytes.Buffer{}
		//pts := int64(pkt.H264PTS.Microseconds())
		//binary.Write(&buf, binary.LittleEndian, pts)
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
