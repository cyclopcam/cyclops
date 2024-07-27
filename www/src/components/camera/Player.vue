<script setup lang="ts">
import type { CameraInfo } from "@/camera/camera";
import { onMounted, onUnmounted, watch, ref } from "vue";
import { VideoStreamer } from "./videoDecode";

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

function imgStyle(): any {
	return {
		"border-radius": borderRadius(),
	}
}

function videoStyle(): any {
	return {
		"border-radius": borderRadius(),
	}
}

watch(() => props.play, (newVal, oldVal) => {
	if (newVal) {
		streamer.play(videoElementID());
	} else {
		stop();
	}
})


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

	if (props.play)
		streamer.play(videoElementID());
})
</script>

<template>
	<div class="container">
		<video class="video" :id="videoElementID()" autoplay :poster="streamer.posterURL()" @play="onPlay"
			@pause="onPause" @click="onClick" :style="videoStyle()" />
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

.livenessCanvas {
	pointer-events: none;
	position: absolute;
	top: 0;
	left: 0;
	width: 1px;
	height: 1px;
}
</style>