import type { CameraInfo } from "@/camera/camera";
import { AnalysisState } from "@/camera/nn";

// SYNC-WEBSOCKET-COMMANDS
export enum WSMessage {
	Pause = "pause",
	Resume = "resume",
}

export enum Codecs {
	H264 = "h264",
	H265 = "h265",
}

export class ParsedPacket {
	constructor(
		public codec: Codecs,
		public video: Uint8Array,
		public recvID: number,
		public duration?: number
	) { }
}

type OnMessageCallback = (msg: AnalysisState | ParsedPacket) => void;

// Manages the I/O portion of video streaming
export class VideoStreamerServerIO {
	camera: CameraInfo;
	ws: WebSocket;
	onMessage: OnMessageCallback;

	backlogDone = false;
	nVideoPackets = 0;
	nBytes = 0;
	lastRecvID = 0;
	firstPacketTime = 0;
	lastCodec = 0;

	constructor(camera: CameraInfo, onMessage: OnMessageCallback, wsUrl: string) {
		this.camera = camera;
		this.onMessage = onMessage;

		this.ws = new WebSocket(wsUrl);
		this.ws.binaryType = "arraybuffer";
		this.ws.addEventListener("message", (event) => {
			let msg = this.decodeWebSocketMessage(event.data);
			if (msg) {
				this.onMessage(msg);
			}
		});
		this.ws.addEventListener("error", (e) => {
			console.log("Video streamer WebSocket Error");
		});

	}

	close() {
		this.ws.close();
	}

	decodeWebSocketMessage(data: string | ArrayBuffer): AnalysisState | ParsedPacket | null {
		if (typeof data === "string") {
			return this.parseStringMessage(data);
		} else {
			return this.parseVideoFrame(data as ArrayBuffer);
		}
	}

	parseVideoFrame(data: ArrayBuffer): ParsedPacket | null {
		let input = new Uint8Array(data);
		let dv = new DataView(input.buffer);

		let now = new Date().getTime();
		if (this.nVideoPackets === 0) {
			this.firstPacketTime = now;
		}

		let headerSize = dv.getUint32(0, true);
		let codec32 = dv.getUint32(4, false); // "H264" or "H265", in big endian byte order so that it looks pretty on the wire, and left-to-right in hex as 0x48323634 or 0x48323635
		let flags = dv.getUint32(8, true);
		let recvID = dv.getUint32(12, true);
		let backlog = (flags & 1) !== 0;
		//console.log("pts", pts);
		let video = input.subarray(headerSize);
		let logPacketCount = false; // SYNC-LOG-PACKET-COUNT

		if (this.lastRecvID !== 0 && recvID !== this.lastRecvID + 1) {
			console.log(`recvID ${recvID} !== lastRecvID ${this.lastRecvID} + 1 (normal when resuming playback after pause)`);
		}

		this.nBytes += input.length;
		this.nVideoPackets++;

		if (!backlog && !this.backlogDone) {
			let nKB = this.nBytes / 1024;
			let kbPerSecond = (1000 * this.nBytes / (now - this.firstPacketTime)) / 1024;
			console.log(`backlogDone in ${now - this.firstPacketTime} ms. ${nKB.toFixed(1)} KB over ${this.nVideoPackets} packets which is ${kbPerSecond.toFixed(0)} KB/second`);
			this.backlogDone = true;
		}

		if (logPacketCount && this.nVideoPackets % 30 === 0) {
			console.log(`${this.camera.name} received ${this.nVideoPackets} packets`);
		}

		// It is better to inject a little bit of frame duration (as opposed to leaving it undefined),
		// because it reduces the jerkiness of the video that we see, presumably due to network and/or camera jitter
		let normalDuration = 1000 / this.camera.ld.fps;

		// This is a naive attempt at forcing the player to catch up to realtime, without introducing
		// too much jitter. I'm not sure if it actually works.
		// OK.. interesting.. I left my system on play for a long time (eg 1 hour), and when I came back,
		// the camera was playing daytime, although it was already night time outside. So *somewhere*, we are
		// adding a gigantic buffer. I haven't figured out how to figure out where that is.
		// OK.. I think that the above situation was a case mentioned in the comments at the top of this file.
		// In other words, the player was still receiving frames, but not presenting them. It buffered them
		// all up, and when the view was made visible again, it started playing them, and obviously they
		// were way behind realtime by that stage. I fixed this by pausing the stream when the view is
		// hidden.
		normalDuration *= 0.99;

		// Try various things to reduce the motion to photons latency. The latency is right now is about 1
		// second, and it's very obvious when you see your neural network detection box walk ahead of your body.
		// Setting duration=undefined to every frame doesn't improve the situation. You get a ton of jank, but
		// latency is still around 2 seconds.

		//if (nVideoPackets % 3 === 0)
		//	backlog = true;
		//backlog = true;

		// during backlog catchup, we leave duration undefined, which causes the player to catch up
		// as fast as it can (which is precisely what we want).

		this.lastRecvID = recvID;

		let codec = Codecs.H264;
		if (codec32 === 0x48323634) {
			codec = Codecs.H264;
		} else if (codec32 === 0x48323635) {
			codec = Codecs.H265;
		} else {
			if (codec32 !== this.lastCodec) {
				console.warn(`Unknown codec 0x${codec32.toFixed(16)} for camera ${this.camera.id}`);
			}
			this.lastCodec = codec32;
			return null;
		}
		if (codec32 !== this.lastCodec) {
			console.log(`Codec ${codec} for camera ${this.camera.id}`);
		}
		this.lastCodec = codec32;

		return new ParsedPacket(
			codec,
			video,
			recvID,
			backlog ? undefined : normalDuration
		);
	}

	parseStringMessage(msg: string): AnalysisState | null {
		let j = JSON.parse(msg);
		if (j.type === "detection") {
			return AnalysisState.fromJSON(j.detection);
		}
		return null;
	}

}