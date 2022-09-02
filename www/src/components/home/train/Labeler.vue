<script setup lang="ts">
import type { Recording } from '@/recording/recording';
import { encodeQuery } from '@/util/util';
import { onMounted, ref } from 'vue';
import VideoTimeline from '../../core/VideoTimeline.vue';
import Cropper from '../../core/Cropper.vue';

// It was too painful to make this a true top-level route,
// so I moved it back to a child component, where we bring
// recording in through a prop.

let props = defineProps<{
	recording: Recording
}>()

const defaultDuration = 1.0; // use 1.0 instead of 0 so we don't have to worry about div/0

let video = ref(null);

let duration = ref(defaultDuration);
let cropStart = ref(0);
let cropEnd = ref(defaultDuration);
let seekPosition = ref(0);

function videoElement(): HTMLVideoElement {
	return video.value! as HTMLVideoElement
}

function videoURL(): string {
	return '/api/record/video/LD/' + props.recording.id + '?' + encodeQuery({ 'seekable': '1' });
}

function onSeek(t: number) {
	seekPosition.value = t;
	videoElement().currentTime = t;
}

function onCropStart(v: number) {
	cropStart.value = v;
	onSeek(v);
}

function onCropEnd(v: number) {
	cropEnd.value = v;
	onSeek(v);
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
	//console.log("duration", videoElement().duration);
})

</script>

<template>
	<div class="labelerRoot" @contextmenu="onContextMenu">
		<video v-if="recording.id !== 0" ref="video" :src="videoURL()" class="video"
			@loadedmetadata="onLoadVideoMetadata" @loadeddata="onLoadVideoData" />
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

.cropper {
	width: $videoWidth;
}

.timeline {
	width: $videoWidth;
}
</style>
