<script setup lang="ts">
import type { Recording } from '@/recording/recording';
import { encodeQuery } from '@/util/util';
import { onMounted, ref } from 'vue';
import VideoTimeline from '../../core/VideoTimeline.vue';

// WebCodecs would be great here, but it's too early (no safari support)

let props = defineProps<{
	recording: Recording
}>()

const defaultDuration = 1.0; // use 1.0 instead of 0 so we don't have to worry about div/0

let video = ref(null);
let canvas = ref(null);
let duration = ref(defaultDuration);
let cropStart = ref(0);
let seekPosition = ref(0);
let cropEnd = ref(defaultDuration);

// This doesn't improve seeking on mobile - it still only seeks to keyframes
let useCanvas = ref(false);

function videoElement(): HTMLVideoElement {
	return video.value! as HTMLVideoElement
}

function videoStyle(): any {
	return {
		"visibility": useCanvas.value ? "hidden" : undefined,
		"position": useCanvas.value ? "absolute" : undefined,
	}
}

function videoURL(): string {
	return '/api/record/video/LD/' + props.recording.id + '?' + encodeQuery({ 'seekable': '1' });
}

function onSeek(t: number) {
	seekPosition.value = t;
	videoElement().currentTime = t;
	if (useCanvas.value) {
		drawFrameNow();
	}
}

function drawFrameNow() {
	let cv = canvas.value! as HTMLCanvasElement;
	let ctx = cv.getContext("2d")!;
	let vd = video.value! as HTMLVideoElement;
	cv.width = cv.width;
	ctx.drawImage(vd, 0, 0);
	ctx.strokeStyle = '#d00';
	ctx.lineWidth = 1;
	//ctx.fillStyle = '#0d0';
	//ctx.fillRect(0, 0, 50, 50);
	ctx.strokeRect(5, 5, 10, 10);
}

function onLoadVideoData() {
	console.log("onLoadVideoData", videoElement().duration);
}

function onLoadVideoMetadata() {
	let el = videoElement();
	console.log("onLoadVideoMetadata", el.duration);
	if (!isNaN(el.duration)) {
		duration.value = el.duration;
		cropStart.value = 0;
		cropStart.value = el.duration;
		seekPosition.value = 0;
		onSeek(0);
		//seekPosition.value = el.duration / 2;
		//seekPosition.value = el.duration;
	}
}

function onContextMenu(ev: Event) {
	console.log("onContextMenu");
	// This is vital to prevent annoying context menu popups on long press on mobile
	ev.preventDefault();
	//return false;
}

onMounted(() => {
	//console.log("duration", videoElement().duration);
	if (useCanvas.value) {
		let cv = canvas.value! as HTMLCanvasElement;
		cv.width = 320;
		cv.height = 240;
	}
})

</script>

<template>
	<div class="labelerRoot" @contextmenu="onContextMenu">
		<video ref="video" :src="videoURL()" class="video" :style="videoStyle()" @loadedmetadata="onLoadVideoMetadata"
			@loadeddata="onLoadVideoData" />
		<canvas v-if="useCanvas" ref="canvas" class="canvas" />
		<video-timeline class="timeline" :duration="duration" :seek-position="seekPosition" :crop-start="cropStart"
			:crop-end="cropEnd" @seek="onSeek" />
		<!--
		<button @click="onSeek1">seek1</button>
		<button @click="onSeek2">seek2</button>
		-->
	</div>
</template>

<style lang="scss" scoped>
@import '@/assets/vars.scss';

.labelerRoot {
	display: flex;
	flex-direction: column;
	padding: 60px 10px;
	box-sizing: border-box;

	width: 400px;
	align-items: center;

	@media (max-width: $mobileCutoff) {
		width: calc(100vw);
		// mobile styles
	}
}

$videoWidth: 340px;
$videoHeight: 250px;

.video {
	width: $videoWidth;
	height: $videoHeight;
	object-fit: fill;
	border-radius: 2px;
}

.canvas {
	width: $videoWidth;
	height: $videoHeight;
}

.timeline {
	width: $videoWidth;
}
</style>
