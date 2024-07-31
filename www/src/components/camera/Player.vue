<script setup lang="ts">
import type { CameraInfo } from "@/camera/camera";
import { onMounted, onUnmounted, watch, ref, reactive } from "vue";
import { VideoStreamer } from "./videoDecode";
import SeekBar from "./SeekBar.vue";
import { SeekBarContext } from "./seekBarContext";

// See videoDecode.ts for an explanation of how this works

let props = defineProps<{
	camera: CameraInfo,
	play: boolean,
	round?: boolean,
	size?: string,
}>()
let emits = defineEmits(['click']);

let showLivenessCanvas = true;
let livenessCanvas = ref(null);
let overlayCanvas = ref(null);
let streamer = new VideoStreamer(props.camera);
let seekBar = reactive(new SeekBarContext(props.camera.id));

function videoElementID(): string {
	return 'vplayer-camera-' + props.camera.id;
}

function onClick() {
	console.log("Player.vue onClick");
	emits('click');
}

function onPlay() {
	// For resuming play when our browser tab has been deactivated, and then reactivated.
	console.log("video element onPlay event");
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
	if (newVal) {
		streamer.play(videoElementID());
	} else {
		stop();
	}
})

async function testLoadHistory() {
	let tiles = await SeekBarContext.downloadTiles(props.camera.id, 300, new Date(2024, 6, 28, 6, 0, 0), new Date(2024, 6, 28, 14, 0, 0));
	//for (let t of tiles) {
	//	console.log(t.level, t.tileIdx, Object.keys(t.classes));
	//	for (let cls in t.classes) {
	//		console.log(t.classes[cls].length);
	//	}
	//}
}

onUnmounted(() => {
	streamer.close();
})

onMounted(() => {
	let liveCanvas: HTMLCanvasElement | null = null;
	if (showLivenessCanvas) {
		liveCanvas = livenessCanvas.value! as HTMLCanvasElement;
	}
	streamer.setDOMElements(overlayCanvas.value! as HTMLCanvasElement, liveCanvas);
	streamer.posterURLUpdateTimer();

	//console.log("On init cameraID = ", props.camera.id);
	//seekBar.cameraID = props.camera.id;
	//console.log("seekBar.cameraID = ", seekBar.cameraID);
	seekBar.seekToNow();
	//testLoadHistory();

	if (props.play)
		streamer.play(videoElementID());
})
</script>

<template>
	<div class="container">
		<div class="videoContainer">
			<video class="video" :id="videoElementID()" autoplay :poster="streamer.posterURL()" @play="onPlay"
				@pause="onPause" @click="onClick" :style="videoStyle()" />
			<canvas ref="overlayCanvas" class="overlay" :style="imgStyle()" />
			<canvas v-if="showLivenessCanvas" ref="livenessCanvas" class="livenessCanvas" />
		</div>
		<seek-bar class="seekBar" :style="bottomStyle()" :camera="camera" :context="seekBar" />
	</div>
</template>

<style lang="scss" scoped>
$seekBarHeight: 10%;

.container {
	width: 100%;
	height: 100%;
	position: relative;
	//border: solid 1px #00d;
}

.videoContainer {
	width: 100%;
	height: calc(100% - $seekBarHeight);
	position: relative;
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