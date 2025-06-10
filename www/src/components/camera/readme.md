# Decoders

To decode h265/hevc, you need either WebCodec or native Java/Swift support. We
can't rely on WebCodec because we sometimes have to connect to the server over
insecure http. I tried WebCodec briefly via localhost, but couldn't get it to
work, and gave up, because I anyway wanted to focus on the Java/Swift route. But
WebCodec can definitely be made to work, just need to figure out what I'm doing
wrong.

Initially I planned on always having the WebSocket run through Typescript. In
other words, the plan was to have VideoStreamerIO manage the IO. But when
implementing the Java decoder, I realized that it's more efficient to have the
WebSocket be handled on the Java side, because it's one less memcpy (i.e.
copying the codec packet from JS to Java would incur an extra copy).
