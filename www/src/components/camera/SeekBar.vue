<script setup lang="ts">
import type { CameraInfo } from '@/camera/camera';
import { onMounted, ref } from 'vue';
import { SeekBarContext } from './seekBarContext';
import { MaxTileLevel } from './eventTile';
import { clamp } from '@/util/util';

let props = defineProps<{
	camera: CameraInfo,
	context: SeekBarContext,
}>()

interface Point {
	id: number;
	x1: number;
	y1: number;
	x2: number;
	y2: number;
}

let canvas = ref(null);
let points: Point[] = [];
let zoomAtPinchStart = 0;

function poll() {
	if (!canvas.value) {
		// end poll - canvas has been destroyed
		return;
	}
	if (props.context.endTimeIsNow) {
		let canv = canvas.value! as HTMLCanvasElement;
		props.context.seekToNow();
		props.context.render(canv);
	}
	setTimeout(poll, 5000);
}

function onWheel(e: WheelEvent) {
	e.preventDefault();
	props.context.zoomLevel += (e.deltaY / 100) * 0.3;
	props.context.zoomLevel = clamp(props.context.zoomLevel, 0, MaxTileLevel + 2);
	props.context.render(canvas.value! as HTMLCanvasElement);
}

function onPointerDown(e: PointerEvent) {
	let canv = canvas.value! as HTMLCanvasElement;
	canv.setPointerCapture(e.pointerId);
	console.log("capture pointer", e.pointerId);
	points.push({ id: e.pointerId, x1: e.offsetX, x2: e.offsetX, y1: e.offsetY, y2: e.offsetY });
	if (points.length === 2) {
		zoomAtPinchStart = props.context.zoomLevel;
	}
}

function onPointerUp(e: PointerEvent) {
	let canv = canvas.value! as HTMLCanvasElement;
	canv.releasePointerCapture(e.pointerId);
	console.log("release pointer", e.pointerId);
	points = [];
}

function onPointerMove(e: PointerEvent) {
	let canv = canvas.value! as HTMLCanvasElement;
	//e.preventDefault();
	//console.log("pointer move", e.pointerId);
	if (points.length !== 2) {
		return;
	}
	for (let p of points) {
		if (p.id === e.pointerId) {
			p.x2 = e.offsetX;
			p.y2 = e.offsetY;
		}
	}
	let orgDistance = Math.hypot(points[0].x1 - points[1].x1, points[0].y1 - points[1].y1);
	let newDistance = Math.hypot(points[0].x2 - points[1].x2, points[0].y2 - points[1].y2);
	//console.log("relative scale", newDistance / orgDistance);
	props.context.zoomLevel = zoomAtPinchStart - Math.log2(newDistance / orgDistance);
	props.context.render(canv);
}

onMounted(() => {
	let canv = canvas.value! as HTMLCanvasElement
	props.context.render(canv);
	poll();
});

</script>

<template>
	<canvas ref="canvas" class="seekBar" @wheel="onWheel" @pointerdown="onPointerDown" @pointerup="onPointerUp"
		@pointermove="onPointerMove" />
</template>

<style lang="scss" scoped>
.seekBar {
	box-sizing: border-box;
	border: solid 1px #000;
	border-top-width: 0;
	background-color: #111;
	touch-action: none; // crucial for pinch zoom to work
}
</style>
