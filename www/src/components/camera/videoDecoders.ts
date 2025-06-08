import { natCreateVideoDecoder, natDecodeVideoPacket, natDestroyVideoDecoder, natNextVideoFrame, type NativeDecoderID } from "@/nativeOut";
import { Codecs } from "@/camera/camera";
import JMuxer from "jmuxer";

export class ParsedPacket {
	constructor(
		public codec: Codecs,
		public video: Uint8Array,
		public recvID: number,
		public keyframe: boolean,
		public backlog: boolean,
		public duration?: number,
	) { }
}

// Our own video decoder interface (different from the native VideoDecoder API, which we can't use over an insecure http connection).
export interface CyVideoDecoder {
	useNextFrame: boolean; // Whether the decoder uses nextFrame() to return frames (false for JMuxer, true otherwise).

	// Shutdown the decoder and release resources.
	close(): void;

	// Feed the decoder a packet.
	decode(packet: ParsedPacket): void;

	// Get the next frame from the decoder, or null if no frame is available.
	nextFrame(): Promise<ImageBitmap | null>;
}

// Create a JMuxer-based video decoder.
// The nextFrame() method always returns null, because JMuxer decodes directly into a video element.
export function createJMuxer(videoElement: string | HTMLVideoElement): CyVideoDecoder {
	let packetQueue: ParsedPacket[] = [];
	let muxer: JMuxer;
	let isReady = false;

	// It IS critical that we wait for the muxer to be ready before feeding it packets.
	// The readme of JMuxer doesn't indicate this, but it's impirically clear that it is.
	let onReady = () => {
		// Feed the muxer the queue of packets that we received before the muxer was ready.
		isReady = true;
		for (let packet of packetQueue) {
			muxer.feed({ video: packet.video });
		}
		packetQueue = [];
	};

	muxer = new JMuxer({
		node: videoElement,
		mode: "video",
		debug: false,
		// OK.. we want to leave FPS unspecified, so that we can control it per-frame, for backlog catchup.
		// If we do specify FPS here, then it becomes Max FPS, and consequently max speedup during backlog catchup.
		//fps: 60,
		maxDelay: 200,
		//flushingTime: 100, // jsmuxer basically runs as setInterval(() => flushFrames(), flushingTime)
		flushingTime: 50, // jsmuxer basically runs as setInterval(() => flushFrames(), flushingTime)
		onReady: onReady,
		onError: () => {
			console.log("jmuxer onError");
		},
		onMissingVideoFrames: () => {
			console.log("jmuxer onMissingVideoFrames");
		},
		onMissingAudioFrames: () => {
			console.log("jmuxer onMissingAudioFrames");
		},
	} as any);

	return {
		useNextFrame: false,

		close() {
			muxer.destroy();
		},

		decode(packet: ParsedPacket) {
			if (!isReady) {
				packetQueue.push(packet);
			} else {
				muxer.feed({ video: packet.video });
			}
		},

		async nextFrame(): Promise<ImageBitmap | null> {
			// JMuxer decodes directly into the video element.
			return null;
		},
	};
}

export async function createNativeAppVideoDecoder(codec: Codecs, width: number, height: number): Promise<CyVideoDecoder> {
	let decoderID = await natCreateVideoDecoder(codec, width, height);
	return {
		useNextFrame: true,
		close: () => {
			natDestroyVideoDecoder(decoderID);
			decoderID = "";
		},
		decode: (packet: ParsedPacket) => {
			natDecodeVideoPacket(decoderID, packet.video);
		},
		nextFrame: async (): Promise<ImageBitmap | null> => {
			return natNextVideoFrame(decoderID);
		},
	};
}
