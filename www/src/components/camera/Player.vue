<script setup lang="ts">
import type { CameraInfo } from "@/camera/camera";
import { onMounted, onUnmounted, watch, ref, reactive } from "vue";
import { VideoStreamer } from "./videoDecode";
import SeekBar from "./SeekBar.vue";
import { SeekBarContext } from "./seekBarContext";
import { debounce } from "@/util/util";

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
//let afterSeekHDTimer = 0;
let seekDebounceTimer = 0;

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
		//clearTimeout(afterSeekHDTimer);
		seekBar.reset();
		seekBarRenderKick.value++;
		streamer.play(videoElementID());
	} else {
		stop();
	}
})

function onSeekEnd() {
	//if (streamer.seekResolution === 'HD') {
	//	// The most recently seeked-to image was an HD image, so don't do anything else
	//	return;
	//}
	clearTimeout(seekDebounceTimer);
	streamer.seekTo(streamer.seekOverlayToMS, 'HD');
}

//function afterSeekLoadHD() {
//}

function seekToNoDelay(seekTo: number) {
	streamer.seekTo(seekTo, 'LD');
	emits('seek', seekTo);
	//clearTimeout(afterSeekHDTimer);
	//afterSeekHDTimer = window.setTimeout(afterSeekLoadHD, 200);
}

//let seekDebounce = debounce((seekTo: number) => {
//	seekToNoDelay(seekTo);
//}, 30);
function seekDebounce(seekTo: number) {
	clearTimeout(seekDebounceTimer);
	seekDebounceTimer = window.setTimeout(() => {
		seekToNoDelay(seekTo);
	}, 30);
}

// This is how we notice that the user wants to seek to a new position
watch(() => seekBar.desiredSeekPosMS, (newVal, oldVal) => {
	if (newVal === 0) {
		// Seek to now, so basically disable seek and go back to displaying latest frame,
		// or possibly a return to live stream.
		return;
	}
	//console.log("Seek to ", newVal);
	if (streamer.hasCachedSeekFrame(newVal, "LD")) {
		seekToNoDelay(newVal);
	} else {
		seekDebounce(newVal);
	}
})

onUnmounted(() => {
	//clearTimeout(afterSeekHDTimer);
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
			<div class="iconContainer flexCenter" @click="onClick">
				<div v-if="!play" :class="{ playIcon: iconIsPlay(), recordIcon: iconIsRecord() }">
				</div>
			</div>
		</div>
		<seek-bar class="seekBar" :style="bottomStyle()" :camera="camera" :context="seekBar"
			:renderKick="seekBarRenderKick" @seekend="onSeekEnd" />
	</div>
</template>

<style lang="scss" scoped>
// HACK! CameraItem.vue has to match this.
$seekBarHeight: 10%;

.container {
	//width: 100%;
	//height: 100%;
	position: relative;
	border: solid 1px #000;
	border-radius: 5px;
	//box-shadow: 0px 0px 2px rgba(255, 255, 255, 0.4), 0px 0px 7px rgba(255, 255, 255, 0.2);
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
	//height: calc(100% - $seekBarHeight);
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