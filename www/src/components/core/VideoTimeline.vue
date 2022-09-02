<script setup lang="ts">

// VideoTimeline is used to:
// 1. Seek
// 2. Set start and end times of an event (to discard a chunk of frames from the start and end of the video))

import { onMounted, onUnmounted, ref } from 'vue';

// all units are seconds

let props = defineProps<{
	duration: number,
	seekPosition: number,
	transparent?: boolean, // If true, then we assume we are preceded by a <video> element, and place ourselve accordingly
}>()
let emits = defineEmits(['seek']);

let seekContainer = ref(null);
let seekDot = ref(null);
let seekGrabber = ref(null);
let mountedAt = ref(0);
let isMounted = ref(false);
let isDestroyed = ref(false);
let isSeeking = ref(false);
let seekDownPosition = ref(0);
let seekDownScreenX = ref(0);
let seekSecondsPerPixel = ref(0);
let myTop = ref(0);

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

// Place ourselves on top of the <video> element that precedes us.
// Assumes our parent is position:relative, and we are position:absolute
function adjustPosition() {
	if (isDestroyed.value) {
		return;
	}
	let sinceInceptionMS = new Date().getTime() - mountedAt.value;
	let tickIntervalMS = sinceInceptionMS < 1000 ? 30 : 200;
	setTimeout(adjustPosition, tickIntervalMS);

	if (!props.transparent) {
		return;
	}
	let self = seekContainer.value! as HTMLElement;
	if (!self) {
		return;
	}
	let parent = self.parentElement;
	let previous = self.previousElementSibling;
	if (parent && previous) {
		let parentR = parent.getBoundingClientRect();
		let previousR = previous.getBoundingClientRect();
		myTop.value = previousR.bottom - parentR.top - 30;
	}
}

function style(): any {
	return {
		"top": myTop.value !== 0 ? myTop.value + "px" : undefined,
	}
}

onMounted(() => {
	mountedAt.value = new Date().getTime();
	isMounted.value = true;
	adjustPosition();
})

onUnmounted(() => {
	isDestroyed.value = true;
})

</script>

<template>
	<div ref="seekContainer" :class="{ seekContainer: true, opaque: !transparent, transparent: transparent }"
		:style="style()" @pointerdown="onSeekDownFar">
		<div class="line" />
		<div ref="seekGrabber" class="grabber" :style="grabberStyle()" @pointerdown="onSeekDown" @pointerup="onSeekUp"
			@pointermove="onSeekMove">
			<div ref="seekDot" class="grabberIcon"></div>
		</div>
	</div>
</template>
	
<style lang="scss" scoped>
@import '@/assets/vars.scss';

$grabberSmallRadius: 6px;

.seekContainer {
	touch-action: none; // vital to prevent scrolling on mobile
	user-select: none;

	width: 100%;
	height: 30px;
	position: relative;
	display: flex;
	align-items: center;
}

.opaque {
	border-radius: 5px;
	background-color: #e3e3e3;
	box-shadow: inset 1px 1px 5px rgba(0, 0, 0, 0.1);
}

.transparent {
	position: absolute;
}

.line {
	touch-action: none; // vital to prevent scrolling on mobile
	user-select: none;

	width: 100%;
	height: 1px;
	background-color: rgba(255, 255, 255, 0.2);
	position: absolute;
}

.grabber {
	touch-action: none; // vital to prevent scrolling on mobile
	user-select: none;

	height: 100%;
	width: 20px;
	position: absolute;
	cursor: pointer;
	top: 0px;
	//background-color: rgba(127, 255, 0, 0.5);
	@include flexCenter();
}

.grabberIcon {
	touch-action: none; // vital to prevent scrolling on mobile
	user-select: none;

	width: $grabberSmallRadius * 2;
	height: $grabberSmallRadius * 2;
	background-color: rgb(255, 255, 255, 1);
	border-radius: 15px;
	border: solid 1px rgb(0, 0, 0, 0.7);
	box-shadow: 1px 1px 3px rgba(0, 0, 0, 0.3);
}
</style>
	