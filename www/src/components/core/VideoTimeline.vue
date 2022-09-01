<script setup lang="ts">

// VideoTimeline is used to:
// 1. Seek
// 2. Set start and end times of an event (to discard a chunk of frames from the start and end of the video))

import { onMounted, ref } from 'vue';

// all units are seconds

let props = defineProps<{
	duration: number,
	seekPosition: number,
	cropStart: number,
	cropEnd: number,
}>()
let emits = defineEmits(['seek']);

let seekContainer = ref(null);
let seekDot = ref(null);
let seekGrabber = ref(null);
let isMounted = ref(false);
let isSeeking = ref(false);
let seekDownPosition = ref(0);
let seekDownScreenX = ref(0);
let seekSecondsPerPixel = ref(0);

//let canvas: HTMLCanvasElement | null = null;

function grabberStyle(): any {
	if (!isMounted.value) {
		return {};
	}
	let container = (seekContainer.value! as HTMLElement).getBoundingClientRect();
	let grabber = (seekGrabber.value! as HTMLElement).getBoundingClientRect();
	let dot = (seekDot.value! as HTMLElement).getBoundingClientRect();
	let w = container.width - dot.width;
	seekSecondsPerPixel.value = props.duration / w;
	//console.log(`container.width: ${container.width}, dot.with: ${dot.width}, seekSecondsPerPixel: ${seekSecondsPerPixel.value} = ${props.duration} / ${w}`);
	return {
		"left": ((w * props.seekPosition / props.duration) - grabber.width / 2 + dot.width / 2) + 'px',
	}
}

function containerPixelsToSeconds(clientX: number): number {
	let container = (seekContainer.value! as HTMLElement).getBoundingClientRect();
	let dot = (seekDot.value! as HTMLElement).getBoundingClientRect();
	let start = dot.width / 2;
	return ((clientX - container.left) - start) * seekSecondsPerPixel.value;
}

// User clicked inside the grabber 
// Don't move the grabber initially.
// All seeking here is entirely based on the delta of movement after the finger went down.
function onSeekDown(ev: PointerEvent) {
	ev.stopPropagation();
	seekDownPosition.value = props.seekPosition;
	startSeek(ev);
}

// User clicked outside the grabber 
// Move the grabber instantly to the clicked location, and then proceed with dragging.
function onSeekDownFar(ev: PointerEvent) {
	ev.preventDefault();
	seekDownPosition.value = containerPixelsToSeconds(ev.clientX);
	startSeek(ev);
	onSeekMove(ev);
}

function startSeek(ev: PointerEvent) {
	isSeeking.value = true;
	let g = seekGrabber.value! as HTMLElement;
	//console.log("capturing pointer ", ev.pointerId);
	g.setPointerCapture(ev.pointerId);
	seekDownScreenX.value = ev.clientX;
}

function onSeekUp(ev: PointerEvent) {
	ev.preventDefault();
	isSeeking.value = false;
	let g = seekGrabber.value! as HTMLElement;
	g.releasePointerCapture(ev.pointerId);
}

function onSeekMove(ev: PointerEvent) {
	ev.preventDefault();
	if (isSeeking.value) {
		let deltaPx = ev.clientX - seekDownScreenX.value;
		let v = seekDownPosition.value + seekSecondsPerPixel.value * deltaPx;
		//console.log(`onSeekMove deltaPx: ${deltaPx},.clientX: ${ev.clientX}, ds: ${seekSecondsPerPixel.value * deltaPx}`);
		v = Math.max(v, 0);
		v = Math.min(v, props.duration);
		//console.log("seek", v);
		emits('seek', v);
	}
}

onMounted(() => {
	isMounted.value = true;
	//canvas = document.createElement("canvas");
	//canvas.width = 320;
	//canvas.height = 240;
})

</script>

<template>
	<div class="timelineRoot">
		<div class="cropContainer">
		</div>
		<div ref="seekContainer" class="seekContainer" @pointerdown="onSeekDownFar">
			<div class="line" />
			<div ref="seekGrabber" class="grabber" :style="grabberStyle()" @pointerdown="onSeekDown"
				@pointerup="onSeekUp" @pointermove="onSeekMove">
				<div ref="seekDot" class="grabberIcon"></div>
			</div>
		</div>
	</div>
</template>
	
<style lang="scss" scoped>
@import '@/assets/vars.scss';

.timelineRoot {
	display: flex;
	flex-direction: column;
	user-select: none;
}

.cropContainer {
	width: 100%;
	height: 40px;
}

$grabberSmallRadius: 3px;

.seekContainer {
	width: 100%;
	height: 40px;
	background-color: rgb(227, 227, 227);
	position: relative;
	display: flex;
	align-items: center;
	touch-action: none; // vital to prevent scrolling on mobile
	border-radius: 7px;
	box-shadow: inset 1px 1px 5px rgba(0, 0, 0, 0.1);
}

.line {
	width: 100%;
	height: 1px;
	background-color: rgb(172, 172, 172);
	position: absolute;
}

.grabber {
	height: 100%;
	width: 30px;
	position: absolute;
	cursor: pointer;
	top: 0px;
	//background-color: rgba(127, 255, 0, 0.5);
	@include flexCenter();
	touch-action: none; // vital to prevent scrolling on mobile
}

.grabberIcon {
	width: $grabberSmallRadius * 2;
	height: 30px;
	background-color: hsl(224, 90%, 45%);
	border-radius: 5px;
	border: solid 1px hsl(0, 0%, 100%);
	box-shadow: 1px 1px 3px rgba(0, 0, 0, 0.3);
}
</style>
	