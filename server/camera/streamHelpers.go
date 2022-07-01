package camera

// A generic message loop that should cater for most streams
func RunStandardStream(ch StreamSinkChan, sink StandardStreamSink) {
	closed := false
	for !closed {
		select {
		case msg := <-ch:
			switch msg.Type {
			case StreamMsgTypeClose:
				sink.Close()
				closed = true
				break
			case StreamMsgTypePacket:
				//fmt.Printf("StreamMsgTypePacket\n")
				sink.OnPacketRTP(msg.Packet)
			}
		}
	}
}
