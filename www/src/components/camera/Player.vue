<script setup lang="ts">
import type { CameraInfo, Resolution } from "@/camera/camera";
import { onMounted, onUnmounted, watch, ref, reactive } from "vue";
import { VideoStreamer } from "./videoDecode";
import SeekBar from "./SeekBar.vue";
import { SeekBarContext } from "./seekBarContext";

// See videoDecode.ts for an explanation of how this works

let props = defineProps<{
	camera: CameraInfo,
	play: boolean,
	width: string,
	height: string,
	icon?: string, // 'play', 'record' (default = play)
	round?: boolean,
}>()
let emits = defineEmits(['playpause', 'seek']);

let showLivenessCanvas = true;
let livenessCanvas = ref(null);
let overlayCanvas = ref(null);
let streamer = new VideoStreamer(props.camera);
let seekBar = reactive(new SeekBarContext(props.camera.id));
let seekBarRenderKick = ref(0);
let seekDebounceTimer = 0;
let lastSeekAt = 0;
let lastSeekTo = 0;
let lastSeekFetchAt = 0;

// This is only useful if the camera is not showing anything (i.e. we can't connect to it),
// but how to detect that? I guess we need an API for that.
let showCameraName = ref(false);

function videoElementID(): string {
	return 'vplayer-camera-' + props.camera.id;
}

function onClick() {
	console.log("Player.vue onClick");
	emits('playpause');
}

function onPlay() {
	// For resuming play when our browser tab has been deactivated, and then reactivated.
	// We also get this message once initial playing starts.
	console.log("Player.vue onPlay");
	streamer.resumePlay();
}

function onPause() {
	console.log("Player.vue onPause");
	streamer.pause();
}

function stop() {
	console.log("Player.vue stop");
	streamer.stop();
}

function borderRadius(): string | undefined {
	return props.round ? "5px" : undefined;
}

function iconIsPlay() { return (props.icon ?? "play") === "play"; }
function iconIsRecord() { return (props.icon ?? "play") === "record"; }

function containerStyle(): any {
	return {
		"width": props.width,
		"height": props.height,
		"border-color": props.play ? "#00a" : "#000",
	}
}

function topStyle(): any {
	return {
		"border-top-left-radius": borderRadius(),
		"border-top-right-radius": borderRadius(),
	}
}

function bottomStyle(): any {
	return {
		"border-bottom-left-radius": borderRadius(),
		"border-bottom-right-radius": borderRadius(),
	}
}

function imgStyle(): any {
	return topStyle();
}

function videoStyle(): any {
	return topStyle();
}

watch(() => props.camera, (newVal, oldVal) => {
	console.log("New cameraID = ", newVal.id);
	seekBar.cameraID = newVal.id;
})

watch(() => props.play, (newVal, oldVal) => {
	console.log(`Player.vue watch(props.play) newVal = ${newVal}`);
	if (newVal) {
		seekBar.reset();
		seekBarRenderKick.value++;
		streamer.play(videoElementID());
	} else {
		stop();
	}
})

function onSeekEnd() {
	clearTimeout(seekDebounceTimer);
	streamer.seekTo(streamer.seekOverlayToMS, 'hd', false);
}

function seekToNoDelay(seekTo: number, resolution: Resolution, keyframeOnly: boolean) {
	streamer.seekTo(seekTo, resolution, keyframeOnly);

	// This emit is how we end up stopping/pausing the live stream (if it's currently busy).
	// The 'seek' event is picked up by Monitor.vue, which then stops the live stream.
	emits('seek', seekTo);
}

function seekDebounce(seekTo: number) {
	// These two variables must be determined dynamically, based on how fast the
	// user is moving the seek bar, and how zoomed in we are. But mostly I think,
	// based on how fast the bar is moving.
	let nowMS = (new Date()).getTime();
	let sinceLastSeek = nowMS - lastSeekAt;
	let distanceSinceLastSeek = Math.abs(seekTo - lastSeekTo);

	// seekSpeed is how fast the user is seeking around, in ms per ms (i.e. NOT pixels, which we probably also want to use)
	let seekSpeed = distanceSinceLastSeek / sinceLastSeek;
	lastSeekAt = nowMS;
	lastSeekTo = seekTo;

	// secondsPerPixel is the seek bar's zoom level
	let secondsPerPixel = seekBar.secondsPerPixel();

	// These constants here are all just empirical thumbsucks
	let keyframeOnly = true;
	let delay = 30;
	if (secondsPerPixel < 2 || seekSpeed < 8) {
		keyframeOnly = false;
	}

	// It would be nice to algorithmically determine the maxFetchesPerSecond. I'm thinking of
	// something along the lines of TCP. For example, you could keep trying to fetch at a slightly
	// higher rate, and if you determine that you're unable to receive frames at that rate, then
	// you bring your matchFetchesPerSecond down, so that you're only just barely exceeding your
	// observed max rate.
	let maxFetchesPerSecond = 1;
	if (seekSpeed < 5) {
		maxFetchesPerSecond = 10;
	} else if (seekSpeed < 20) {
		maxFetchesPerSecond = 2;
	}
	//console.log(`Zoom seconds per pixel: ${secondsPerPixel}, seekSpeed: ${seekSpeed}, maxFetchesPerSecond: ${maxFetchesPerSecond}`);

	let intervalMS = 1000 / maxFetchesPerSecond;
	let sinceLastSeekFetch = nowMS - lastSeekFetchAt;
	if (sinceLastSeekFetch > intervalMS) {
		// If we're without our FPS budget, just kick off the fetch without any delay.
		// What's nice about this code path, is we don't fall victim to that thing with
		// a debounce, where you're moving the seek point slowly but consistently, so
		// every single movement keeps getting debounced. Kicking the can down the road.
		// With this path, we at least maintain some FPS.
		clearTimeout(seekDebounceTimer);
		seekToNoDelay(seekTo, 'ld', keyframeOnly);
		lastSeekFetchAt = nowMS;
	} else {
		// BUT, if we're in a low FPS regime (eg high seekSpeed), then debounce is still a great thing
		// to have, so that when your finger rests on your desired destination, you still get the frame
		// after a few MS. Without this, you might move your finger fast to where you want to be, and
		// then the system will just sit there, waiting for you to move slowly, before it will fetch
		// another frame.
		clearTimeout(seekDebounceTimer);
		seekDebounceTimer = window.setTimeout(() => {
			lastSeekFetchAt = nowMS;
			seekToNoDelay(seekTo, 'ld', keyframeOnly);
		}, delay);
	}

}

// This is how we notice that the user wants to seek to a new position
watch(() => seekBar.desiredSeekPosMS, (newVal, oldVal) => {
	if (newVal === 0) {
		// Seek to now, so basically disable seek and go back to displaying latest frame,
		// or possibly a return to live stream.
		return;
	}
	//console.log("Seek to ", newVal);
	if (streamer.hasCachedSeekFrame(newVal, 'hd')) {
		seekToNoDelay(newVal, 'hd', false);
	} else if (streamer.hasCachedSeekFrame(newVal, 'ld')) {
		seekToNoDelay(newVal, 'ld', false);
	} else {
		seekDebounce(newVal);
	}
})

onUnmounted(() => {
	clearTimeout(seekDebounceTimer);
	streamer.close();
})

onMounted(() => {
	let liveCanvas: HTMLCanvasElement | null = null;
	if (showLivenessCanvas) {
		liveCanvas = livenessCanvas.value! as HTMLCanvasElement;
	}
	streamer.setDOMElements(overlayCanvas.value! as HTMLCanvasElement, liveCanvas);
	streamer.posterURLUpdateTimer();

	seekBar.panToNow();

	if (props.play)
		streamer.play(videoElementID());
})
</script>

<template>
	<div class="container" :style="containerStyle()">
		<div class="videoContainer">
			<video class="video" :id="videoElementID()" autoplay :poster="streamer.posterURL()" @play="onPlay"
				@pause="onPause" :style="videoStyle()" />
			<canvas ref="overlayCanvas" class="overlay" :style="imgStyle()" />
			<canvas v-if="showLivenessCanvas" ref="livenessCanvas" class="livenessCanvas" />
			<div v-if="showCameraName" class="name">{{ camera.name }}</div>
			<div class="iconContainer flexCenter noselect" @click="onClick">
				<div v-if="!play" :class="{ playIcon: iconIsPlay(), recordIcon: iconIsRecord() }">
				</div>
			</div>
		</div>
		<seek-bar class="seekBar" :style="bottomStyle()" :camera="camera" :context="seekBar"
			:renderKick="seekBarRenderKick" @seekend="onSeekEnd" />
	</div>
</template>

<style lang="scss" scoped>
$seekBarHeight: 10%;

.container {
	position: relative;
	border: solid 1px #000;
	border-radius: 5px;
}

.videoContainer {
	width: 100%;
	height: calc(100% - $seekBarHeight);
	position: relative;
}

.iconContainer {
	position: absolute;
	left: 0px;
	top: 0px;
	width: 100%;
	height: 100%;
	cursor: pointer;
}

.playIcon {
	background-repeat: no-repeat;
	background-size: 30px 30px;
	background-position: center;
	width: 30px;
	height: 30px;
	background-image: url("@/icons/play-circle-outline.svg");
	//filter: invert(1) drop-shadow(1px 1px 3px rgba(0, 0, 0, 0.9));
}

.playIcon:hover {
	filter: invert(1) drop-shadow(0px 0px 1px rgb(183, 184, 255)) drop-shadow(1.5px 1.5px 3px rgba(0, 0, 0, 0.9));
}

.recordIcon {
	background-color: #e00;
	width: 16px;
	height: 16px;
	border-radius: 100px;
	border: solid 2px #fff;
	animation-name: pulse;
	animation-duration: 0.6s;
	animation-iteration-count: infinite;
	animation-direction: alternate;
	animation-timing-function: cubic-bezier(0.1, 0, 0.9, 1); // https://cubic-bezier.com/#0,.2,1,.8
}

@keyframes pulse {
	from {
		transform: scale(1);
		opacity: 1;
	}

	to {
		transform: scale(1.15);
		opacity: 0.5;
	}
}

.name {
	position: absolute;
	right: 4px; // put name on the right, because video-encoded time display is usually on the top left
	top: 4px;
	font-size: 10px;
	color: #fff;
	filter: drop-shadow(0px 0px 2px #000);
	border-radius: 2px;
	padding: 2px 4px;
	background: rgba(0, 0, 0, 0.2)
}

.video {
	width: 100%;
	height: 100%;
	// This screws up the aspect ratio, but I feel like it's the right UI tradeoff for consistency of the video widgets.
	// Without this, on Chrome on Linux, as soon as the player starts decoding frames, it adjusts itself to the actual
	// aspect ratio of the decoded video stream, and this usually leaves a letter box in our UI. Normally I hate distorting
	// aspect ratio, but in this case I believe it's the best option.
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

.seekBar {
	position: absolute;
	left: 0;
	bottom: 0;
	width: 100%;
	height: $seekBarHeight;
	border-bottom-left-radius: 5px;
	border-bottom-right-radius: 5px;
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