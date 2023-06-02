<script setup lang="ts">
import type { CameraInfo } from "@/camera/camera";
import { encodeQuery } from "@/util/util";
import { globals } from "@/globals";
import JMuxer from "jmuxer";
import { onMounted, onUnmounted, watch, ref } from "vue";
import { AnalysisState, DetectionResult, COCOClasses } from "@/camera/nn";

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
Because if the user re-activates the tab, then she will want the video to resume
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
Once a <video> element has received the first frame, then it will stop using the poster image,
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

let props = defineProps<{
	camera: CameraInfo,
	play: boolean,
	round?: boolean,
	size?: string,
	allowRecord?: boolean,
}>()
let emits = defineEmits(['click']);

let posterUrlCacheBreaker = ref(Math.round(Math.random() * 1e9));

let showPosterImageInOverlay = ref(true);

let showLivenessCanvas = ref(true);
let livenessCanvas = ref(null);

let overlayCanvas = ref(null);

let muxer: JMuxer | null = null;
let ws: WebSocket | null = null;
let backlogDone = false;
let nVideoPackets = 0;
let nBytes = 0;
let lastRecvID = 0;
let firstPacketTime = 0;
let isPaused = false;
let posterURLUpdateFrequencyMS = 5 * 1000; // When the page is active, we update our poster URL every X seconds
let posterURLTimerID: any = 0;
//let lastDetection = new Map<number, DetectionResult>(); // key is camera ID
let lastDetection = new AnalysisState();

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

function parseVideoFrame(data: ArrayBuffer): parsedPacket {
	let input = new Uint8Array(data);
	let dv = new DataView(input.buffer);

	let now = new Date().getTime();
	if (nVideoPackets === 0) {
		firstPacketTime = now;
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

	if (lastRecvID !== 0 && recvID !== lastRecvID + 1) {
		console.log(`recvID ${recvID} !== lastRecvID ${lastRecvID} + 1 (normal when resuming playback after pause)`);
	}

	nBytes += input.length;
	nVideoPackets++;

	if (!backlog && !backlogDone) {
		let bytesPerSecond = 1000 * nBytes / (now - firstPacketTime);
		console.log(`backlogDone in ${now - firstPacketTime} ms. ${nBytes} bytes over ${nVideoPackets} packets which is ${bytesPerSecond} bytes/second`);
		backlogDone = true;
	}

	if (logPacketCount && nVideoPackets % 30 === 0) {
		console.log(`${props.camera.name} received ${nVideoPackets} packets`);
	}

	// It is better to inject a little bit of frame duration (as opposed to leaving it undefined),
	// because it reduces the jerkiness of the video that we see, presumably due to network and/or camera jitter
	let normalDuration = 1000 / props.camera.ld.fps;

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

	lastRecvID = recvID;

	return {
		video: video,
		recvID: recvID,
		duration: backlog ? undefined : normalDuration,
		//duration: undefined,
	};
}

function parseStringMessage(msg: string) {
	let j = JSON.parse(msg);
	if (j.type === "detection") {
		let detection = AnalysisState.fromJSON(j.detection);
		lastDetection = detection;
		updateOverlay();
	}
}

function canShowRecord(): boolean {
	return props.allowRecord ?? true;
}

function videoElementID(): string {
	return 'vplayer-camera-' + props.camera.id;
}

function createMuxer(onReady: () => void): JMuxer {
	return new JMuxer({
		node: videoElementID(),
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

function play() {
	let isPlaying = muxer !== null;
	console.log("play(). isPlaying: " + (isPlaying ? "yes" : "no"));
	showPosterImageInOverlay.value = false;
	updateOverlay();
	if (isPlaying)
		return;

	isPaused = false;

	let scheme = window.location.origin.startsWith("https") ? "wss://" : "ws://";
	let socketURL = scheme + window.location.host + "/api/ws/camera/stream/LD/" + props.camera.id;
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
			let player = document.getElementById(videoElementID()) as HTMLVideoElement;
			let nFrames = (player as any).webkitDecodedFrameCount;
			console.log(`frames: ${nFrames}, firstPackets.length: ${firstPackets.length}`);
			globals.isFirstVideoPlay = false;
			if (nFrames === 0) {
				console.log(`No frames decoded, so recreating muxer`);
				phase = 1;
				muxer!.destroy();
				muxer = createMuxer(onMuxerReadyPass2);
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

	//let ticker = () => {
	//	let player = document.getElementById(videoElementID()) as HTMLVideoElement;
	//	let nFrames = (player as any).webkitDecodedFrameCount;
	//	console.log(`ticker. frames: ${nFrames}`);
	//	setTimeout(ticker, 1000);
	//}
	//ticker();

	if (globals.isFirstVideoPlay) {
		setTimeout(onMuxerReadyPass1, 500);
	}

	muxer = createMuxer(() => { console.log("muxer ready"); isMuxerReady = true });
	//muxer = createMuxer(onMuxerReadyPass1);

	ws = new WebSocket(socketURL);
	ws.binaryType = "arraybuffer";
	ws.addEventListener("message", function (event) {
		if (isPaused) {
			return;
		}
		if (muxer) {
			if (typeof event.data === "string") {
				parseStringMessage(event.data);
			} else {
				let data = parseVideoFrame(event.data);
				muxer.feed(data);
				if (globals.isFirstVideoPlay && phase === 0) {
					firstPackets.push(data);
				}
				if (showLivenessCanvas.value) {
					invalidateLivenessCanvas();
				}
				nVideoPackets++;
			}
		}
	});

	ws.addEventListener("error", function (e) {
		console.log("Socket Error");
	});
}

function invalidateLivenessCanvas() {
	let can = livenessCanvas.value! as HTMLCanvasElement;
	can.width = 1;
	can.height = 1;
	let cx = can.getContext('2d')!;
	cx.fillStyle = "rgba(0,0,0,0.01)";
	cx.fillRect(0, 0, 1, 1);
}

function onClick() {
	console.log("Player onClick");
	emits('click');
}

function onPlay() {
	console.log("video element onPlay event");

	// For resuming play when our browser tab has been deactivated, and then reactivated.
	showPosterImageInOverlay.value = false;

	//play();
	if (isPaused) {
		isPaused = false;
		sendWSMessage(WSMessage.Resume);
	}
}

function onPause() {
	console.log("onPause");

	showPosterImageInOverlay.value = true;
	resetPosterURL();

	//stop();
	isPaused = true;
	sendWSMessage(WSMessage.Pause);
}

function stop() {
	console.log("Player.vue stop");

	showPosterImageInOverlay.value = true;
	resetPosterURL();

	isPaused = false;
	if (ws) {
		ws.close();
		ws = null;
	}
	if (muxer) {
		muxer.destroy();
		muxer = null;
	}
}

function sendWSMessage(msg: WSMessage) {
	// SYNC-WEBSOCKET-JSON-MSG	
	if (!ws) {
		return;
	}
	ws.send(JSON.stringify({ command: msg }));
}

function posterURL(): string {
	return "/api/camera/latestImage/" + props.camera.id + "?" + encodeQuery({ cacheBreaker: posterUrlCacheBreaker.value });
}

function borderRadius(): string | undefined {
	return props.round ? "5px" : undefined;
}

function imgStyle(): any {
	return {
		"border-radius": borderRadius(),
	}
}

function videoStyle(): any {
	/*
	let width = props.camera.ld.width + "px";
	let height = props.camera.ld.height + "px";
	if (props.size) {
		switch (props.size) {
			case "small":
				width = "200px";
				height = "140px";
				break;
			case "medium":
				width = "320px";
				height = "200px";
				break;
			default:
				console.error(`Unknown camera size ${props.size}`);
		}
	}
	*/

	return {
		//width: width,
		//height: height,
		"border-radius": borderRadius(),
	}
}

watch(() => props.play, (newVal, oldVal) => {
	if (newVal) {
		play();
	} else {
		stop();
	}
})

function resetPosterURL() {
	posterUrlCacheBreaker.value = Math.round(Math.random() * 1e9);
	updateOverlay();
}

function posterURLUpdateTimer() {
	if (document.visibilityState === "visible" && showPosterImageInOverlay.value) {
		resetPosterURL();
	}
	//console.log(`posterURLUpdateTimer ${props.camera.id}`);
	posterURLTimerID = setTimeout(posterURLUpdateTimer, posterURLUpdateFrequencyMS);
}

function drawRawNNObjects(can: HTMLCanvasElement, cx: CanvasRenderingContext2D, detection: DetectionResult) {
	let sx = can.width / detection.imageWidth;
	let sy = can.height / detection.imageHeight;
	for (let d of detection.objects) {
		cx.lineWidth = 2;
		cx.strokeStyle = "#0c0";
		cx.font = '18px sans-serif';
		cx.strokeRect(d.box.x * sx, d.box.y * sy, d.box.width * sx, d.box.height * sy);
		cx.fillStyle = '#fff';
		cx.textAlign = 'left';
		cx.textBaseline = 'top';
		cx.fillText(COCOClasses[d.class], d.box.x * sx, d.box.y * sy);
	}
}

function drawAnalyzedObjects(can: HTMLCanvasElement, cx: CanvasRenderingContext2D, detection: AnalysisState) {
	if (!detection.input)
		return;
	let sx = can.width / detection.input.imageWidth;
	let sy = can.height / detection.input.imageHeight;
	for (let d of detection.objects) {
		if (d.genuine) {
			cx.lineWidth = 4;
			cx.strokeStyle = "#f00";
			cx.font = 'bold 18px sans-serif';
		} else {
			cx.lineWidth = 2;
			cx.strokeStyle = "#fc0";
			cx.font = '18px sans-serif';
		}
		cx.strokeRect(d.box.x * sx, d.box.y * sy, d.box.width * sx, d.box.height * sy);
		cx.fillStyle = '#fff';
		cx.textAlign = 'left';
		cx.textBaseline = 'top';
		cx.fillText(COCOClasses[d.class] + ' ' + d.id, d.box.x * sx, d.box.y * sy);
	}
}

async function updateOverlay() {
	let can = overlayCanvas.value! as HTMLCanvasElement;
	if (!can) {
		return;
	}
	let dpr = window.devicePixelRatio;
	can.width = can.clientWidth * dpr;
	can.height = can.clientHeight * dpr;
	//console.log(`updateOverlay ${can.width}x${can.height}`);

	let cx = can.getContext('2d')!;

	if (showPosterImageInOverlay.value) {
		let r = await fetch(posterURL());
		if (!r.ok)
			return;
		let blob = await r.blob();
		let image = await createImageBitmap(blob);
		cx.drawImage(image, 0, 0, can.width, can.height);

		let jDetections = r.headers.get("X-Analysis");
		if (jDetections) {
			lastDetection = AnalysisState.fromJSON(JSON.parse(jDetections));
		}
		//console.log("detections", r.headers.get("X-Detections"));
	}

	if (lastDetection.cameraID === props.camera.id && lastDetection.input) {
		drawAnalyzedObjects(can, cx, lastDetection);
		//drawRawNNObjects(can, cx, lastDetection.input);
	}
}

onUnmounted(() => {
	clearTimeout(posterURLTimerID);
	stop();
})

onMounted(() => {
	posterURLUpdateTimer();
	if (props.play)
		play();
})
</script>

<template>
	<div class="container">
		<video class="video" :id="'vplayer-camera-' + camera.id" autoplay :poster="posterURL()" @play="onPlay"
			@pause="onPause" @click="onClick" :style="videoStyle()" />
		<!-- <img v-if="showOverlay" class="overlay" :src="posterURL()" :style="imgStyle()" /> -->
		<canvas ref="overlayCanvas" class="overlay" :style="imgStyle()" />
		<canvas v-if="showLivenessCanvas" ref="livenessCanvas" class="livenessCanvas" />
	</div>
</template>

<style lang="scss" scoped>
.container {
	width: 100%;
	height: 100%;
	position: relative;
}

.video {
	width: 100%;
	height: 100%;
	// This screws up the aspect ratio, but I feel like it's the right UI tradeoff for consistency of the video widgets.
	// Without this, on Chrome on Linux, as soon as the player starts decoding frames, it adjusts itself to the actual
	// aspect ratio of the decoded video stream, and this usually leaves a letter box in our UI. Normally I hate distorting
	// aspect ratio, but in this case I actually think it's the best option.
	object-fit: fill;
}

.overlay {
	pointer-events: none;
	position: absolute;
	top: 0;
	left: 0;
	width: 100%;
	height: 100%;
}

.livenessCanvas {
	pointer-events: none;
	position: absolute;
	top: 0;
	left: 0;
	width: 1px;
	height: 1px;
}
</style>