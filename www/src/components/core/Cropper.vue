<script setup lang="ts">

// Cropper is used to build the video cropper

import { clamp } from '@/util/util';
import { onMounted, ref, watch } from 'vue';

// all units are seconds

let props = defineProps<{
	duration: number,
	start: number,
	end: number,
}>()
let emits = defineEmits(['seekStart', 'seekEnd']);

enum Dots {
	None = 0,
	Start = 1,
	End = 2
}

let container = ref(null);
let dotStart = ref(null);
let dotEnd = ref(null);
let isMounted = ref(false);
let dotWidthPx = ref(1);
let busySeeking = ref(Dots.None);
let seekDownSeconds = ref(0);
let seekDownClientX = ref(0);
let pixelsPerSecond = ref(0);
let paddingPx = ref(1);

//watch(() => props.duration, updateMeasurements);

function updateMeasurements(): any {
	let containerR = (container.value! as HTMLElement).getBoundingClientRect();
	let dotR = (dotStart.value! as HTMLElement).getBoundingClientRect();
	dotWidthPx.value = dotR.width;
	pixelsPerSecond.value = (containerR.width - paddingPx.value * 2) / props.duration;
	//console.log("cropper", containerR.width, dotR.width, pixelsPerSecond.value, props.duration);
}

function dotStyle(dot: Dots): any {
	if (!isMounted.value) {
		return {};
	}
	updateMeasurements();
	let seconds = dot === Dots.Start ? props.start : props.end;
	let px = secondsToContainerPx(seconds);
	if (dot === Dots.End) {
		px -= dotWidthPx.value;
	}
	return {
		"left": px + 'px',
	}
}

function fillStyle(): any {
	if (!isMounted.value) {
		return {};
	}
	return {
		"left": secondsToContainerPx(props.start) + "px",
		"width": secondsToContainerPx(props.end) - secondsToContainerPx(props.start) + "px",
	}
}

function secondsToContainerPx(seconds: number): number {
	return paddingPx.value + seconds * pixelsPerSecond.value;
}

function containerPxToSeconds(px: number): number {
	return px / pixelsPerSecond.value - paddingPx.value;
}

function clientXToContainerPx(clientX: number): number {
	let containerR = (container.value! as HTMLElement).getBoundingClientRect();
	return clientX - containerR.left;
}

// User clicked inside the grabber 
// Don't move the grabber initially.
// All seeking here is entirely based on the delta of movement after the finger went down.
function onSeekDown(ev: PointerEvent) {
	//ev.stopPropagation();
	ev.preventDefault();
	let containerPx = clientXToContainerPx(ev.clientX);

	let startPx = secondsToContainerPx(props.start);
	let endPx = secondsToContainerPx(props.end);
	let startPxCenter = secondsToContainerPx(props.start) + dotWidthPx.value / 2;
	let endPxCenter = secondsToContainerPx(props.end) - dotWidthPx.value / 2;

	// figure out which button was clicked on
	let dStart = Math.abs(containerPx - startPx);
	let dEnd = Math.abs(containerPx - endPx);
	let dStartCenter = Math.abs(containerPx - startPxCenter);
	let dEndCenter = Math.abs(containerPx - endPxCenter);
	let dot = dStart <= dEnd ? Dots.Start : Dots.End;

	busySeeking.value = dot;
	let c = container.value! as HTMLElement;
	c.setPointerCapture(ev.pointerId);

	// If the click was close to a grabber, then treat this as a relative movement,
	// so zero initial seek.
	// If the click was far from the grabber, then treat this as an absolute initial
	// seek, and relative thereafter.
	let dMin = Math.min(dStartCenter, dEndCenter);
	if (dMin < dotWidthPx.value * 0.5) {
		// purely relative
		seekDownSeconds.value = dot === Dots.Start ? props.start : props.end;
		seekDownClientX.value = ev.clientX;
		//console.log("Start seek relative", dot);
	} else {
		// jump
		let seekToPx = containerPx;
		if (dot === Dots.End) {
			seekToPx += dotWidthPx.value / 2;
		}
		seekDownSeconds.value = clamp(containerPxToSeconds(seekToPx), 0, props.duration);
		seekDownClientX.value = ev.clientX;
		onSeekMove(ev);
		//console.log("Start seek jump", dot);
	}
}

function onSeekUp(ev: PointerEvent) {
	//console.log("Seek end");
	ev.preventDefault();
	busySeeking.value = Dots.None;
	let c = container.value! as HTMLElement;
	c.releasePointerCapture(ev.pointerId);
}

function onUpOther(ev: PointerEvent) {
	//console.log("up other");
}

function onMenu(ev: Event) {
	//console.log("onMenu");
	ev.preventDefault();
}

function onSeekMove(ev: PointerEvent) {
	//console.log("onSeekMove");
	ev.preventDefault();
	if (busySeeking.value !== Dots.None) {
		let deltaPx = ev.clientX - seekDownClientX.value;
		let v = seekDownSeconds.value + deltaPx / pixelsPerSecond.value;
		//console.log(`onSeekMove deltaPx: ${deltaPx},.clientX: ${ev.clientX}, ds: ${secondsPerPixel.value * deltaPx}`);
		if (busySeeking.value === Dots.Start) {
			v = clamp(v, 0, props.end - 1);
		} else if (busySeeking.value === Dots.End) {
			v = clamp(v, props.start + 1, props.duration);
		}
		if (busySeeking.value === Dots.Start) {
			emits('seekStart', v);
		} else {
			emits('seekEnd', v);
		}
	}
}

onMounted(() => {
	isMounted.value = true;
})

</script>

<template>
	<div ref="container" class="container" @pointerdown="onSeekDown" @pointerup="onSeekUp" @pointermove="onSeekMove">
		<div class="fill" :style="fillStyle()" @pointerup="onUpOther" @contextmenu="onMenu" />
		<div ref="dotStart" class="dot start" :style="dotStyle(Dots.Start)" @pointerup="onUpOther"
			@contextmenu="onMenu">
		</div>
		<div ref="dotEnd" class="dot end" :style="dotStyle(Dots.End)" @pointerup="onUpOther" @contextmenu="onMenu">
		</div>
	</div>
</template>

<style lang="scss" scoped>
@import '@/assets/vars.scss';

$radius: 5px;

.container {
	touch-action: none; // vital to prevent scrolling on mobile
	user-select: none;

	position: relative;
	display: flex;
	align-items: center;
	width: 100%;
	height: 40px;
	background-color: #777;
	border-radius: $radius;
	box-shadow: inset 1px 1px 5px rgba(0, 0, 0, 0.1);
	//cursor: pointer;
}

.fill {
	touch-action: none; // vital to prevent scrolling on mobile
	user-select: none;

	background-color: #d8d8d8;
	position: absolute;
	height: 100%;
	box-sizing: border-box;
	border-top: solid 1px #aaa;
	border-bottom: solid 1px #aaa;
	border-radius: $radius;
}

.dot {
	touch-action: none; // vital to prevent scrolling on mobile
	user-select: none;

	position: absolute;
	width: 32px;
	height: 32px;
	background-size: 32px;
	cursor: pointer;
}

.start {
	background-image: url('@/icons/arrow-bar-to-left.svg');
	border-top-left-radius: $radius;
	border-bottom-left-radius: $radius;
}

.end {
	background-image: url('@/icons/arrow-bar-to-right.svg');
	border-top-right-radius: $radius;
	border-bottom-right-radius: $radius;
}
</style>
