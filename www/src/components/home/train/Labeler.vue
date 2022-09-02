<script setup lang="ts">
import { globals } from '@/globals';
import type { Recording } from '@/recording/recording';
import router from '@/router/routes';
import { encodeQuery } from '@/util/util';
import { onMounted, ref } from 'vue';
import VideoTimeline from '../../core/VideoTimeline.vue';
import Cropper from '../../core/Cropper.vue';

// It was too painful to make this a true top-level route,
// so I moved it back to a child component.

let props = defineProps<{
	//recordingID: string, // Comes from our route, so it must be a string, although it's actually a number
	recording: Recording
}>()

const defaultDuration = 1.0; // use 1.0 instead of 0 so we don't have to worry about div/0

let video = ref(null);
let canvas = ref(null);

//let recording = ref(new Recording()); // Via route
let duration = ref(defaultDuration);
let cropStart = ref(0);
let cropEnd = ref(defaultDuration);
let seekPosition = ref(0);

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

function onCropStart(v: number) {
	cropStart.value = v;
	onSeek(v);
}

function onCropEnd(v: number) {
	cropEnd.value = v;
	onSeek(v);
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
		cropStart.value = el.duration / 4;
		cropEnd.value = el.duration * 3 / 4;
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

onMounted(async () => {
	/*
	// Load via route...
	//Recording.load(router.currentRoute.value.params['recordingID'] as any as number);
	let r = await Recording.load(parseInt(props.recordingID));
	if (r.ok) {
		recording.value = r.value;
	} else {
		globals.networkError = r.err;
	}
	*/

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
		<video v-if="recording.id !== 0" ref="video" :src="videoURL()" class="video" :style="videoStyle()"
			@loadedmetadata="onLoadVideoMetadata" @loadeddata="onLoadVideoData" />
		<canvas v-if="useCanvas" ref="canvas" class="canvas" />
		<div style="height:20px" />
		<div class="instruction">Crop the video to the precise moments when this is happening</div>
		<cropper class="cropper" :duration="duration" :start="cropStart" :end="cropEnd" @seek-start="onCropStart"
			@seek-end="onCropEnd" />
		<div style="height:15px" />
		<!--
		<video-timeline class="timeline" :duration="duration" :seek-position="seekPosition" @seek="onSeek" />
		-->
	</div>
</template>

<style lang="scss" scoped>
@import '@/assets/vars.scss';

.labelerRoot {
	display: flex;
	flex-direction: column;
	align-items: center;

	box-sizing: border-box;
	background-color: #fff;
	padding: 60px 10px;

	width: 400px;

	@media (max-width: $mobileCutoff) {
		width: 100vw;
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

.instruction {
	font-size: 14px;
	margin-bottom: 10px;
	width: $videoWidth;
}

.canvas {
	width: $videoWidth;
	height: $videoHeight;
}

.cropper {
	width: $videoWidth;
}

.timeline {
	width: $videoWidth;
}
</style>
