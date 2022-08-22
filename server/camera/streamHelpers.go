package camera

// A generic message loop that should cater for most streams
func RunStandardStream(sink StandardStreamSink, ch StreamSinkChan) {
	closed := false
	for !closed {
	outerloop:
		select {
		case msg := <-ch:
			switch msg.Type {
			case StreamMsgTypeClose:
				//fmt.Printf("RunStandardStream StreamMsgTypeClose (enter)\n")
				sink.Close()
				closed = true
				break outerloop
			case StreamMsgTypePacket:
				//fmt.Printf("RunStandardStream StreamMsgTypePacket\n")
				sink.OnPacketRTP(msg.Packet)
			}
		}
	}
}
