import type { CameraInfo, Resolution, StreamInfo } from "@/camera/camera";
import { AnalysisState } from "@/camera/nn";
import { globals } from "@/globals";
import { BoxDrawMode, drawAnalyzedObjects } from "./detections";
import { encodeQuery, sleep } from "@/util/util";
import { CachedFrame, FrameCache } from "./frameCache";
import { WSMessage, VideoStreamerIO } from "./videoWebSocket";
import { Codecs } from "@/camera/camera";
import { createJMuxer, type CyVideoDecoder, ParsedPacket } from "./videoDecoders";
import { createWebCodecsDecoder } from "./videoDecoderWebCodec";
import { natWsVideoNextFrame, natWsVideoPlay, natWsVideoStop } from "@/nativeOut";

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

export enum PlayerState {
	Stopped = "stopped", // We have no websocket connection, so we're not receiving frames.
	Playing = "playing", // We have a websocket connection, and we're receiving frames.
	Paused = "paused", // We have a websocket connection, but we've asked it to stop sending us frames.
}

// Use JMuxer to decode video packets so we can render them to a canvas or a <video> element.
// Also, understand our own WebSocket messages which transport video packets.
export class VideoStreamer {
	camera: CameraInfo;

	creatingDecoder = false;
	decoder: CyVideoDecoder | null = null; // either JMuxer or a native decoder (eg on Android).
	decoderReady = false;

	state = PlayerState.Stopped;
	resolution: Resolution = "ld"; // The resolution that we're currently playing. This is set when we call play().

	serverIO: VideoStreamerIO | null = null;
	posterURLUpdateFrequencyMS = 5 * 1000; // When the page is active, we update our poster URL every X seconds
	posterURLTimerID: any = 0;
	lastDetection = new AnalysisState();
	posterUrlCacheBreaker = Math.round(Math.random() * 1e9);
	showPosterImageInOverlay = true;
	videoElementID: string = ""; // The ID of the <video> element that we're using to play the video stream (for h264 with JMuxer).
	videoCanvas: HTMLCanvasElement | null = null; // Used when decoding the video manually (eg native Android)
	overlayCanvas: HTMLCanvasElement | null = null;
	livenessCanvas: HTMLCanvasElement | null = null;
	isUnableToDecodeMessageShown = false;

	seekOverlayToMS = 0; // If not 0, then the overlay canvas is rendered with a keyframe closest to this time. This is for seeking back in time.
	seekIndexNext = 1; // Used to tell if we should discard a fetch (eg if an older seek finished AFTER a newer seek, then discard the older result)
	seekResolution: Resolution = "ld";
	seekImage: ImageBitmap | null = null; // Most recent seek frame
	seekImageIndex = 0; // seekCount at the time when the fetch of this seekImage was initiated
	seekCache: FrameCache;

	nativePlayerID = "";

	constructor(camera: CameraInfo) {
		this.camera = camera;
		this.seekCache = new FrameCache(camera);
	}

	posterURLUpdateTimer() {
		if (document.visibilityState === "visible" && this.showPosterImageInOverlay) {
			this.resetPosterURL();
		}
		//console.log(`posterURLUpdateTimer ${props.camera.id}`);
		this.posterURLTimerID = setTimeout(() => {
			this.posterURLUpdateTimer();
		}, this.posterURLUpdateFrequencyMS);
	}

	close() {
		this.stop();
		clearTimeout(this.posterURLTimerID);
	}

	setDOMElements(videoCanvas: HTMLCanvasElement | null, overlayCanvas: HTMLCanvasElement | null, livenessCanvas: HTMLCanvasElement | null) {
		this.videoCanvas = videoCanvas;
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
		this.isUnableToDecodeMessageShown = false;

		if (this.state === PlayerState.Paused) {
			this.state = PlayerState.Playing;
			this.sendWSMessage(WSMessage.Resume);
		}
	}

	resetPosterState() {
		this.showPosterImageInOverlay = true;
		this.isUnableToDecodeMessageShown = false;
		this.resetPosterURL();
	}

	pause() {
		this.resetPosterState();
		this.state = PlayerState.Paused;
		this.sendWSMessage(WSMessage.Pause);
	}

	stop() {
		this.resetPosterState();
		this.state = PlayerState.Stopped;
		if (this.serverIO) {
			this.serverIO.close();
			this.serverIO = null;
		}
		this.destroyDecoder();
		if (this.nativePlayerID !== '') {
			console.log(`Stopping native player`);
			natWsVideoStop(this.nativePlayerID);
			this.nativePlayerID = '';
		}
	}

	clearSeek() {
		this.seekImage = null;
		this.seekCache.clear();
		this.seekOverlayToMS = 0;
	}

	destroyDecoder() {
		this.decoderReady = false;
		if (this.decoder) {
			this.decoder.close();
			this.decoder = null;
		}
	}

	hasCachedSeekFrame(posMS: number, resolution: Resolution): boolean {
		posMS = this.seekCache.snapToNearestFrame(posMS, resolution, false);
		let cacheKey = FrameCache.makeKey(resolution, posMS);
		return this.seekCache.get(cacheKey) !== undefined;
	}

	async fetchSingleFrame(resolution: Resolution, posMS: number, quality: number, keyframeOnly: boolean): Promise<CachedFrame | null> {
		let cacheKey = FrameCache.makeKey(resolution, posMS);
		let fromCache = this.seekCache.get(cacheKey);
		if (fromCache) {
			return fromCache;
		}
		let seekMode = "";
		if (keyframeOnly) {
			seekMode = "nearestKeyframe";
		}
		let url = `/api/camera/image/${this.camera.id}/${resolution}/${Math.round(posMS)}?quality=${quality}&seekMode=${seekMode}`;
		let r = await fetch(url);
		if (!r.ok) return null;
		let blob = await r.blob();
		// Getting rid of this FPS estimate, because we already know the camera FPS
		//let estimatedFPS = parseInt(r.headers.get("X-Cyclops-FPS") ?? '0', 10);
		let frameTime = parseInt(r.headers.get("X-Cyclops-Frame-Time") ?? "0", 10);
		if (keyframeOnly) {
			this.seekCache.addKeyframeTime(resolution, frameTime);
		}
		let analysis: AnalysisState | undefined;
		let analysisHeader = r.headers.get("X-Analysis");
		if (analysisHeader) {
			analysis = AnalysisState.fromJSON(JSON.parse(analysisHeader));
		}
		return this.seekCache.add(cacheKey, blob, frameTime, analysis);
	}

	async seekTo(posMS: number, resolution: Resolution, keyframeOnly: boolean) {
		posMS = this.seekCache.snapToNearestFrame(posMS, resolution, keyframeOnly);
		this.seekOverlayToMS = posMS;
		this.seekResolution = resolution;
		let myIndex = this.seekIndexNext;
		this.seekIndexNext++;

		// We have two potential API calls to make here, and we want to kick them
		// off in parallel. That's why we structure the fetches as individual
		// promises, so that we can await on them both at the same time.
		// uhh.. I don't understand this comment now!

		let quality = resolution === "ld" ? 70 : 85;
		let fetchFrame = this.fetchSingleFrame(resolution, posMS, quality, keyframeOnly);
		let [cachedFrame] = await Promise.all([fetchFrame]);
		if (!cachedFrame) {
			return;
		}
		let img = await createImageBitmap(cachedFrame.blob);
		if (this.seekImageIndex > myIndex) {
			// A newer image has already been fetched and decoded
			return;
		}
		if (this.state === PlayerState.Playing) {
			// User clicked 'play' to play the live stream while we were waiting for our seek frame, so discard the seek frame.
			return;
		}
		this.lastDetection = cachedFrame.analysis ?? new AnalysisState();
		this.seekImageIndex = myIndex;
		this.seekImage = img;
		this.updateOverlay();
	}

	play(videoElementID: string, res: Resolution) {
		console.log(`VideoStreamer.play(). state: ${this.state}`);
		this.videoElementID = videoElementID;
		this.showPosterImageInOverlay = false;
		this.clearSeek();
		this.updateOverlay();
		if (this.state === PlayerState.Playing) return;

		this.state = PlayerState.Playing;
		this.resolution = res;

		let scheme = window.location.origin.startsWith("https") ? "wss://" : "ws://";
		let socketURL = `${scheme}${window.location.host}/api/ws/camera/stream/${this.camera.id}/${res}`;
		console.log("Play " + socketURL);

		if (this.useNativePlayer()) {
			this.startNativePlayer(socketURL);
		} else {
			this.startOwnPlayer(socketURL);
		}
	}

	get currentStream(): StreamInfo {
		if (this.resolution === "ld") {
			return this.camera.ld;
		} else {
			return this.camera.hd;
		}
	}

	useNativePlayer(): boolean {
		//console.log(`useNativePlayer: globals.isApp: ${globals.isApp}, camera.codec: ${this.currentStream.codec}`);
		return globals.isApp && this.camera.ld.codec !== Codecs.H264;
	}

	// In this mode, Java opens the WebSocket and decodes the frames. It sends them back to us via WebView.postMessage().
	async startNativePlayer(socketURL: string) {
		console.log("Starting native player for " + this.camera.name + " at " + socketURL);
		let stream = this.currentStream;
		try {
			this.nativePlayerID = await natWsVideoPlay(socketURL, stream.codec, stream.width, stream.height);
			this.nativeFramePoller();
		} catch (err) {
			console.error("Failed to start native player for " + this.camera.name + ": " + err);
			this.showUnableToDecodeMessage(stream.codec, err + '');
			return;
		}
	}

	async nativeFramePoller() {
		if (this.state !== PlayerState.Playing || !this.nativePlayerID) {
			console.log(`nativeFramePoller exiting. ${this.state}, ${this.nativePlayerID}`);
			return;
		}

		const frameStart = performance.now();
		let frame = await natWsVideoNextFrame(this.nativePlayerID, this.currentStream.width, this.currentStream.height);
		const frameEnd = performance.now();

		if (this.state !== PlayerState.Playing) {
			// state might've changed during await
			return;
		}

		if (frame) {
			this.drawImageOnVideoCanvas(frame);
		}

		// Calculate time to next frame
		const targetFrameDuration = 1000 / this.currentStream.fps;
		const elapsed = frameEnd - frameStart;
		let nextDelay = targetFrameDuration - elapsed;

		// If we're behind, schedule immediately to try and catch up
		if (nextDelay < 0) nextDelay = 0;

		setTimeout(() => this.nativeFramePoller(), nextDelay);
	}

	startOwnPlayer(socketURL: string) {
		var onWebSocketMessage = (data: ParsedPacket | AnalysisState) => {
			if (this.state !== PlayerState.Playing) {
				return;
			}
			if (data instanceof AnalysisState) {
				this.lastDetection = data;
				this.updateOverlay();
			} else if (data instanceof ParsedPacket) {
				// decodePacket is an async function, but we don't wait for it to return here.
				this.decodePacket(data);
			}
		};
		this.serverIO = new VideoStreamerIO(this.camera, onWebSocketMessage, socketURL);
	}

	async createDecoder(codec: Codecs) {
		if (codec !== Codecs.H264) {
			// I've never managed to get WebCodecs to work, so will continue this experiment further, on different platforms.
			//try {
			//	this.decoder = await createWebCodecsDecoder(codec);
			//} catch (err) {
			//	console.warn(`Failed to create decoder for codec ${codec}:`, err);
			//}
			if (!this.decoder) {
				this.showUnableToDecodeMessage(codec);
				return;
			}
		}

		this.decoder = createJMuxer(this.videoElementID);
	}

	async decodePacket(packet: ParsedPacket) {
		// If a packet arrives while we're still busy creating the decoder, then just
		// sleep until that other decodePacket invocation finishes.
		while (this.creatingDecoder) {
			await sleep(100);
		}

		if (!this.decoder) {
			this.creatingDecoder = true;
			try {
				await this.createDecoder(packet.codec);
			}
			finally {
				this.creatingDecoder = false;
			}
		}

		if (this.decoder) {
			this.decoder.decode(packet);
			this.presentFrame();
		}
	}

	async presentFrame() {
		if (!this.decoder) {
			return;
		}
		if (!this.decoder.useNextFrame) {
			// This path is for JMuxer. It decodes directly into the video element.
			// All we need to do is invalidate our liveness canvas, to make sure the
			// browser knows there's new content to render.
			this.invalidateLivenessCanvas();
			return;
		}
		// This is for our own decoder(s), where we need to pull frames once they're ready
		let bmp = await this.decoder.nextFrame();
		if (bmp) {
			this.drawImageOnVideoCanvas(bmp);
		}
	}

	drawImageOnVideoCanvas(image: ImageBitmap) {
		let can = this.videoCanvas!;
		if (can.width !== image.width || can.height !== image.height) {
			can.width = image.width;
			can.height = image.height;
		}
		let cx = can.getContext('2d')!;
		cx.drawImage(image, 0, 0, can.width, can.height);
	}

	sendWSMessage(msg: WSMessage) {
		// SYNC-WEBSOCKET-JSON-MSG
		if (!this.serverIO) {
			return;
		}
		this.serverIO.ws.send(JSON.stringify({ command: msg }));
	}

	invalidateLivenessCanvas() {
		if (!this.livenessCanvas) {
			return;
		}
		let can = this.livenessCanvas;
		can.width = 1;
		can.height = 1;
		let cx = can.getContext("2d")!;
		cx.fillStyle = "rgba(0,0,0,0.01)";
		cx.fillRect(0, 0, 1, 1);
	}

	showUnableToDecodeMessage(codec: Codecs, err?: string) {
		if (!this.videoCanvas || this.isUnableToDecodeMessageShown) {
			return;
		}
		this.isUnableToDecodeMessageShown = true;
		this.videoCanvas.width = 500;
		this.videoCanvas.height = 360;
		let cx = this.videoCanvas.getContext('2d')!;
		cx.fillStyle = "black";
		cx.fillRect(0, 0, this.videoCanvas.width, this.videoCanvas.height);
		cx.fillStyle = "white";
		cx.font = "30px Arial";
		cx.textAlign = "center";
		let x = this.videoCanvas.width / 2;
		let y = this.videoCanvas.height / 2 - 10;
		cx.fillText(`Unable to decode ${codec} video.`, x, y);
		if (!err) {
			err = `Use the mobile app instead.`;
		}
		cx.fillText(err, x, y + 40);
	}

	async updateOverlay() {
		let can = this.overlayCanvas;
		if (!can) {
			return;
		}
		let dpr = window.devicePixelRatio;
		let canvasScale = 1;

		// This function is async, so it's important that we don't clear the canvas
		// until we're ready to paint.
		// The point of resetCanvasOnce() is to only clear it a single time, regardless
		// of which async operation finishes first.
		let isCanvasReset = false;
		let resetCanvasOnce = (minWidth: number, minHeight: number) => {
			// TS thinks 'can' can be null here, but I can't see how that could happen.
			if (!isCanvasReset && can) {
				let width = Math.ceil(can.clientWidth * dpr);
				let height = Math.ceil(can.clientHeight * dpr);
				if (width < minWidth || height < minHeight) {
					// This is used when zooming in on an HD frame, so that we don't lose resolution.
					canvasScale = Math.max(minWidth / width, minHeight / height);
					width = minWidth;
					height = minHeight;
				}
				//console.log(`Resetting canvas to ${width} x ${height} (min ${minWidth} x ${minHeight})`);
				can.width = width;
				can.height = height;
			}
			isCanvasReset = true;
		};

		let cx = can.getContext('2d')!;
		let isHDSeek = false;

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
			resetCanvasOnce(image.width, image.height);
			cx.drawImage(image, 0, 0, can.width, can.height);
			isHDSeek = this.seekResolution === 'hd';

			if (r) {
				let jAnalysis = r.headers.get("X-Analysis");
				if (jAnalysis) {
					this.lastDetection = AnalysisState.fromJSON(JSON.parse(jAnalysis));
				}
				//console.log("detections", r.headers.get("X-Detections"));
			}
		}

		if (this.lastDetection.cameraID === this.camera.id && this.lastDetection.input) {
			resetCanvasOnce(0, 0);
			if (!isHDSeek) {
				let boxDraw = isHDSeek ? BoxDrawMode.Thin : BoxDrawMode.Regular;
				drawAnalyzedObjects(can, cx, this.lastDetection, boxDraw);
			}
			//drawRawNNObjects(can, cx, lastDetection.input);
		}

		//console.log(`updateOverlay ${can.width}x${can.height}`);
	}
}


/*
let phase = 0;
let isMuxerReady = false;

let onMuxerReadyPass2 = () => {
	console.log("onMuxerReadyPass2");
	phase = 2;
};

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
			this.destroyDecoder();
			this.createDecoder(videoElementID, onMuxerReadyPass2);
		}
	} else {
		// Keep waking up again, until we have received 10 packets.
		setTimeout(onMuxerReadyPass1, 200);
	}
};

if (globals.isFirstVideoPlay) {
	setTimeout(onMuxerReadyPass1, 500);
}

this.createDecoder(videoElementID, () => {
	console.log("muxer ready");
	isMuxerReady = true;
});
*/
