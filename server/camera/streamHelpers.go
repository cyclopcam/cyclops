package camera

// A generic message loop that should cater for most streams
func RunStandardStream(stream *Stream, sink StandardStreamSink, ch StreamSinkChan) {
	// When returning, inform stream that we're done with cleanup
	defer stream.RemoveSink(ch)

	for {
		select {
		case msg := <-ch:
			switch msg.Type {
			case StreamMsgTypeClose:
				//fmt.Printf("RunStandardStream StreamMsgTypeClose (enter)\n")
				// Allow sink object to perform cleanup
				sink.Close()
				return
			case StreamMsgTypePacket:
				//fmt.Printf("RunStandardStream StreamMsgTypePacket\n")
				sink.OnPacketRTP(msg.Packet)
			}
		}
	}
}
