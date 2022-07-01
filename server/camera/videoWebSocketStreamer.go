package camera

import (
	"github.com/aler9/gortsplib"
	"github.com/gorilla/websocket"
)

type VideoWebSocketStreamer struct {
	incoming StreamSinkChan
}

func (s *VideoWebSocketStreamer) OnConnect(stream *Stream) (StreamSinkChan, error) {
	return s.incoming, nil
}

func (s *VideoWebSocketStreamer) OnPacketRTP(ctx *gortsplib.ClientOnPacketRTPCtx) {
}

func (s *VideoWebSocketStreamer) Run(conn *websocket.Conn, stream *Stream) {
	stream.ConnectSink(s, false)
	defer stream.RemoveSink(s)
	defer conn.Close()

	// Or do we have two goroutines, one for receiving from WS, and one for receiving from Stream?
	// The only reason we receive from WS is to be notified of websocket closure.

	closed := false
	for !closed {
		select {
		case msg := <-s.incoming:
			switch msg.Type {
			case StreamMsgTypeClose:
				closed = true
				break
			case StreamMsgTypePacket:
				s.OnPacketRTP(msg.Packet)
			}
		}
	}
}
