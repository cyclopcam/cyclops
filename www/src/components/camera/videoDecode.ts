import type { CameraInfo } from "@/camera/camera";
import { AnalysisState } from "@/camera/nn";
import JMuxer from "jmuxer";
import { globals } from "@/globals";
import { drawAnalyzedObjects } from "./detections";
import { encodeQuery } from "@/util/util";
import { FrameCache } from "./frameCache";

/*

Player is for playing a live camera stream.

A websocket feeds us h264 packets, and we use jmuxer to feed them into
a <video> object.

If our browser tab is made inactive, then we receive a pause event from the <video> element.
When our tab is re-activated, we get a play event.
We need to take care of this in some ways:
1. When paused, the server must stop sending us frames, because it's a waste of bandwidth.
2. When reactivated, we must immediately start playing again.

If we just do nothing, then two bad things happen:
1. We waste bandwidth
2. The browser seems to buffer up ALL of the frames that got sent while we
   were in the pause state, and when we resume, it plays those frames first.
   So basically, the video is no longer realtime. This makes sense if you're
   playing a movie, but not for a live view.

Why don't we just stop the video if we receive a pause event?
Because if a user re-activates the tab, they will want the video to resume
playing, without having to click "play" again.

Weird Android WebView issue
---------------------------
On Android the following happens:
The first time that we try to play a video, something goes wrong, and the video
doesn't play. We see no error messages - JMuxer is happy. However, when I look
at the Android logs (via logcat), I see the following:

VideoCapabilities   org.cyclops   W  Unsupported mime image/vnd.android.heic
VideoCapabilities   org.cyclops   W  Unrecognized profile/level 0/3 for video/mpeg2
VideoCapabilities   org.cyclops   W  Unrecognized profile/level 0/3 for video/mpeg2
cr_MediaCodecUtil   org.cyclops   E  Decoder for type video/av01 is not supported on this device [requireSoftware=false, requireHardware=true].

This is strange, because it only happens the first time we try to play a video.
On subsequent attempts, everything works.

My workaround for this is to wait for the JMuxer onReady event, then recreate
JMuxer, play back our original packets, and continue from there. This does create
a noticable pause before playing back the first time, and it only works about
90% of the time. I'm hoping this just goes away with time, and subsequent
Android/Chrome updates.

UPDATE: There's more to this story.
Now I'm seeing the above error messages, and then success after that. Without
having to do anything. During this time, the camera streams went from black & white
to color. That may have something to do with it. Also, I once saw a JMuxer
"missing frames" event. I'm beginning to wonder if my "backlog" mechanism is
the true culprit here.

Poster Image
------------
Once a <video> element has received the first frame, it will stop using the poster image,
and instead use the first video frame for its poster. We don't want this. We want to keep
updating our poster image every few seconds, even if the video is paused.

Since we always show our own overlay for a poster image, why do we even bother
setting the "poster" attribute on the <video> element? The reason is because 
without this, when we first hit play, then the video element will become white.
With a poster image, it continues to display the poster image until the video
stream starts playing.

Liveness Canvas
---------------
In some situations on Android, frames will be decoded, but the WebView will not
update itself. The workaround is to draw a 1x1 pixel canvas on top of the video
element, and draw to this canvas every time we receive a frame. This happens so
often of my Xiaomi Redmit Note 9 Pro, that I always enable the liveness canvas.

*/


// SYNC-WEBSOCKET-COMMANDS
enum WSMessage {
	Pause = "pause",
	Resume = "resume",
}

interface parsedPacket {
	video: Uint8Array,
	recvID: number,
	duration?: number
}

// Use JMuxer to decode video packets so we can render them to a canvas or a <video> element.
// Also, understand our own WebSocket messages which transport video packets.
export class VideoStreamer {
	camera: CameraInfo;
	muxer: JMuxer | null = null;
	ws: WebSocket | null = null;
	backlogDone = false;
	nVideoPackets = 0;
	nBytes = 0;
	lastRecvID = 0;
	firstPacketTime = 0;
	posterURLUpdateFrequencyMS = 5 * 1000; // When the page is active, we update our poster URL every X seconds
	posterURLTimerID: any = 0;
	lastDetection = new AnalysisState();
	isPaused = false;
	posterUrlCacheBreaker = Math.round(Math.random() * 1e9);
	showPosterImageInOverlay = true;
	overlayCanvas: HTMLCanvasElement | null = null;
	livenessCanvas: HTMLCanvasElement | null = null;

	seekOverlayToMS = 0; // If not 0, then the overlay canvas is rendered with a keyframe closest to this time. This is for seeking back in time.
	seekIndexNext = 1; // Used to tell if we should discard a fetch (eg if an older seek finished AFTER a newer seek, then discard the older result)
	seekResolution: 'LD' | 'HD' = 'LD';
	seekImage: ImageBitmap | null = null; // Most recent seek frame
	seekImageIndex = 0; // seekCount at the time when the fetch of this seekImage was initiated
	seekCache = new FrameCache();

	constructor(camera: CameraInfo) {
		this.camera = camera;
	}

	posterURLUpdateTimer() {
		if (document.visibilityState === "visible" && this.showPosterImageInOverlay) {
			this.resetPosterURL();
		}
		//console.log(`posterURLUpdateTimer ${props.camera.id}`);
		this.posterURLTimerID = setTimeout(() => { this.posterURLUpdateTimer() }, this.posterURLUpdateFrequencyMS);
	}

	close() {
		this.stop();
		clearTimeout(this.posterURLTimerID);
	}

	setDOMElements(overlayCanvas: HTMLCanvasElement | null, livenessCanvas: HTMLCanvasElement | null) {
		this.overlayCanvas = overlayCanvas;
		this.livenessCanvas = livenessCanvas;
	}

	posterURL(): string {
		return "/api/camera/latestImage/" + this.camera.id + "?" + encodeQuery({ cacheBreaker: this.posterUrlCacheBreaker });
	}

	resetPosterURL() {
		this.posterUrlCacheBreaker = Math.round(Math.random() * 1e9);
		this.updateOverlay();
	}

	resumePlay() {
		// For resuming play when our browser tab has been deactivated, and then reactivated.
		this.showPosterImageInOverlay = false;

		if (this.isPaused) {
			this.isPaused = false;
			this.sendWSMessage(WSMessage.Resume);
		}
	}

	pause() {
		this.showPosterImageInOverlay = true;
		this.resetPosterURL();

		this.isPaused = true;
		this.sendWSMessage(WSMessage.Pause);
	}

	stop() {
		this.showPosterImageInOverlay = true;
		this.resetPosterURL();

		this.isPaused = false;
		if (this.ws) {
			this.ws.close();
			this.ws = null;
		}
		if (this.muxer) {
			this.muxer.destroy();
			this.muxer = null;
		}
	}

	clearSeek() {
		this.seekImage = null;
		this.seekCache.clear();
		this.seekOverlayToMS = 0;
	}

	hasCachedSeekFrame(posMS: number, resolution: string): boolean {
		posMS = this.seekCache.suggestNearestFrameTime(posMS);
		let cacheKey = FrameCache.makeKey(this.camera.id, resolution, posMS);
		return this.seekCache.get(cacheKey) !== undefined;
	}

	async seekTo(posMS: number, resolution: 'LD' | 'HD') {
		posMS = this.seekCache.suggestNearestFrameTime(posMS);
		this.seekOverlayToMS = posMS;
		this.seekResolution = resolution;
		let myIndex = this.seekIndexNext;
		this.seekIndexNext++;

		let quality = resolution === 'LD' ? '70' : '85';
		let cacheKey = FrameCache.makeKey(this.camera.id, resolution, posMS);
		let fromCache = this.seekCache.get(cacheKey);
		let blob: Blob | null = null;
		if (fromCache) {
			blob = fromCache.blob;
		} else {
			let url = `/api/camera/image/${this.camera.id}/${resolution}/${posMS}?quality=${quality}`;
			let r = await fetch(url);
			if (!r.ok)
				return;
			blob = await r.blob();
			let estimatedFPS = parseInt(r.headers.get("X-Cyclops-FPS") ?? '0', 10);
			let frameTime = parseInt(r.headers.get("X-Cyclops-Frame-Time") ?? '0', 10);
			this.seekCache.add(cacheKey, blob, estimatedFPS, frameTime);
		}
		let img = await createImageBitmap(blob);
		if (this.seekImageIndex > myIndex) {
			// A newer image has already been fetched and decoded
			return;
		}
		this.seekImageIndex = myIndex;
		this.seekImage = img;
		this.updateOverlay();
	}

	destroyMuxer() {
		if (this.muxer) {
			this.muxer.destroy();
			this.muxer = null;
		}
	}

	play(videoElementID: string) {
		let isPlaying = this.muxer !== null;
		console.log("VideoStreamer.play(). isPlaying: " + (isPlaying ? "yes" : "no"));
		this.showPosterImageInOverlay = false;
		this.clearSeek();
		this.updateOverlay();
		if (isPlaying)
			return;

		this.isPaused = false;

		let scheme = window.location.origin.startsWith("https") ? "wss://" : "ws://";
		let resolution = "LD";
		let socketURL = `${scheme}${window.location.host}/api/ws/camera/stream/${this.camera.id}/${resolution}`;
		console.log("Play " + socketURL);

		let firstPackets: parsedPacket[] = [];
		let phase = 0;
		let isMuxerReady = false;

		let onMuxerReadyPass2 = () => {
			console.log("onMuxerReadyPass2");
			phase = 2;
		}

		let onMuxerReadyPass1 = () => {
			// See the long comment at the top of the page about the "Weird Android Issue".
			// Basically, we're resetting the muxer here, but we only need to do it once per page load.
			if (isMuxerReady && firstPackets.length > 10) {
				let player = document.getElementById(videoElementID) as HTMLVideoElement;
				let nFrames = (player as any).webkitDecodedFrameCount;
				console.log(`frames: ${nFrames}, firstPackets.length: ${firstPackets.length}`);
				globals.isFirstVideoPlay = false;
				if (nFrames === 0) {
					console.log(`No frames decoded, so recreating muxer`);
					phase = 1;
					this.destroyMuxer();
					this.createMuxer(videoElementID, onMuxerReadyPass2);
					// I suspect that my "backlog" mechanism might be at fault.
					// This hack seems to be more robust when I omit the backlog on the
					// 2nd attempt.
					//for (let p of firstPackets) {
					//	muxer.feed(p);
					//}
					firstPackets = [];
				}
			} else {
				setTimeout(onMuxerReadyPass1, 200);
			}
		}

		if (globals.isFirstVideoPlay) {
			setTimeout(onMuxerReadyPass1, 500);
		}

		this.createMuxer(videoElementID, () => { console.log("muxer ready"); isMuxerReady = true });

		this.ws = new WebSocket(socketURL);
		this.ws.binaryType = "arraybuffer";
		this.ws.addEventListener("message", (event) => {
			if (this.isPaused) {
				return;
			}
			if (this.muxer) {
				if (typeof event.data === "string") {
					this.parseStringMessage(event.data);
				} else {
					let data = this.parseVideoFrame(event.data);
					this.muxer.feed(data);
					if (globals.isFirstVideoPlay && phase === 0) {
						firstPackets.push(data);
					}
					this.invalidateLivenessCanvas();
					this.nVideoPackets++;
				}
			}
		});

		this.ws.addEventListener("error", (e) => {
			console.log("Socket Error");
		});
	}

	createMuxer(videoElement: string | HTMLVideoElement, onReady: () => void) {
		this.muxer = new JMuxer({
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
			onError: () => { console.log("jmuxer onError"); },
			onMissingVideoFrames: () => { console.log("jmuxer onMissingVideoFrames"); },
			onMissingAudioFrames: () => { console.log("jmuxer onMissingAudioFrames"); },
		} as any);
	}

	parseVideoFrame(data: ArrayBuffer): parsedPacket {
		let input = new Uint8Array(data);
		let dv = new DataView(input.buffer);

		let now = new Date().getTime();
		if (this.nVideoPackets === 0) {
			this.firstPacketTime = now;
		}

		//let foo1 = dv.getUint32(0, true);
		//let foo2 = dv.getUint32(4, true);
		//console.log("foos", foo1, foo2);
		//let pts = dv.getFloat64(0, true);
		let flags = dv.getUint32(0, true);
		let recvID = dv.getUint32(4, true);
		let backlog = (flags & 1) !== 0;
		//console.log("pts", pts);
		let video = input.subarray(8);
		let logPacketCount = false; // SYNC-LOG-PACKET-COUNT

		if (this.lastRecvID !== 0 && recvID !== this.lastRecvID + 1) {
			console.log(`recvID ${recvID} !== lastRecvID ${this.lastRecvID} + 1 (normal when resuming playback after pause)`);
		}

		this.nBytes += input.length;
		this.nVideoPackets++;

		if (!backlog && !this.backlogDone) {
			let bytesPerSecond = 1000 * this.nBytes / (now - this.firstPacketTime);
			console.log(`backlogDone in ${now - this.firstPacketTime} ms. ${this.nBytes} bytes over ${this.nVideoPackets} packets which is ${bytesPerSecond} bytes/second`);
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

		return {
			video: video,
			recvID: recvID,
			duration: backlog ? undefined : normalDuration,
			//duration: undefined,
		};
	}

	parseStringMessage(msg: string) {
		let j = JSON.parse(msg);
		if (j.type === "detection") {
			let detection = AnalysisState.fromJSON(j.detection);
			this.lastDetection = detection;
			this.updateOverlay();
		}
	}

	sendWSMessage(msg: WSMessage) {
		// SYNC-WEBSOCKET-JSON-MSG
		if (!this.ws) {
			return;
		}
		this.ws.send(JSON.stringify({ command: msg }));
	}

	invalidateLivenessCanvas() {
		if (!this.livenessCanvas) {
			return;
		}
		let can = this.livenessCanvas;
		can.width = 1;
		can.height = 1;
		let cx = can.getContext('2d')!;
		cx.fillStyle = "rgba(0,0,0,0.01)";
		cx.fillRect(0, 0, 1, 1);
	}

	async updateOverlay() {
		let can = this.overlayCanvas;
		if (!can) {
			return;
		}
		let dpr = window.devicePixelRatio;

		// This function is async, so it's important that we don't clear the canvas
		// until we're ready to paint.
		// The point of resetCanvasOnce() is to only clear it a single time, regardless
		// of which async operation finishes first.
		let isCanvasReset = false;
		let resetCanvasOnce = () => {
			// TS thinks 'can' can be null here, but I can't see how that could happen.
			if (!isCanvasReset && can) {
				can.width = can.clientWidth * dpr;
				can.height = can.clientHeight * dpr;
			}
			isCanvasReset = true;
		};

		let cx = can.getContext('2d')!;

		if (this.showPosterImageInOverlay || this.seekImage) {
			let image = this.seekImage;
			let r: Response | null = null;
			if (!image) {
				let url = this.posterURL();
				r = await fetch(url);
				if (!r.ok)
					return;
				let blob = await r.blob();
				image = await createImageBitmap(blob);
			}
			resetCanvasOnce();
			cx.drawImage(image, 0, 0, can.width, can.height);

			if (r) {
				let jAnalysis = r.headers.get("X-Analysis");
				if (jAnalysis) {
					this.lastDetection = AnalysisState.fromJSON(JSON.parse(jAnalysis));
				}
				//console.log("detections", r.headers.get("X-Detections"));
			}
		}

		if (this.lastDetection.cameraID === this.camera.id && this.lastDetection.input) {
			resetCanvasOnce();
			drawAnalyzedObjects(can, cx, this.lastDetection);
			//drawRawNNObjects(can, cx, lastDetection.input);
		}

		//console.log(`updateOverlay ${can.width}x${can.height}`);
	}
}