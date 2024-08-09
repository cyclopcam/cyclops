<script setup lang="ts">
import type { CameraInfo } from '@/camera/camera';
import { onMounted, ref } from 'vue';
import { SeekBarContext, SeekBarTransform } from './seekBarContext';
import { MaxTileLevel } from './eventTile';
import { clamp } from '@/util/util';

let props = defineProps<{
	camera: CameraInfo,
	context: SeekBarContext,
}>()

interface Point {
	id: number; // pointer id
	x1: number; // x at finger down
	y1: number; // y at finger down
	x2: number; // latest x finger
	y2: number; // latest y finger
}

let canvas = ref(null);
let grabber = ref(null);
let points: Point[] = []; // when there are 2 points, points[0] is on the left and points[1] is on the right
let zoomAtPinchStart = 0;
let txAtPinchStart = new SeekBarTransform();
//let rightEdgeMSAtPinchStart = 0;

// Convert from CSS pixel (eg from PointerEvent) to our canvas coordinates, which are native device pixels
function pxToCanvas(cssPx: number): number {
	return cssPx * window.devicePixelRatio;
}

function poll() {
	if (!canvas.value) {
		// end poll - canvas has been destroyed
		return;
	}
	if (props.context.panTimeEndIsNow) {
		let canv = canvas.value! as HTMLCanvasElement;
		props.context.panToNow();
		props.context.render(canv);
	}
	setTimeout(poll, 5000);
}

// On desktop, you can scroll with the mouse wheel
function onWheel(e: WheelEvent) {
	//console.log("onWheel", e.deltaX, e.deltaY);
	e.preventDefault();
	props.context.zoomLevel += (e.deltaY / 100) * 0.3;
	props.context.zoomLevel = clamp(props.context.zoomLevel, 0, MaxTileLevel + 2);
	props.context.render(canvas.value! as HTMLCanvasElement);
}

function onPointerDown(e: PointerEvent) {
	//console.log("onPointerDown", e.pointerId);
	let canv = canvas.value! as HTMLCanvasElement
	let grab = grabber.value! as HTMLDivElement;
	grab.setPointerCapture(e.pointerId);
	let ox = pxToCanvas(e.offsetX);
	let oy = pxToCanvas(e.offsetY);
	points.push({ id: e.pointerId, x1: ox, x2: ox, y1: oy, y2: oy });
	if (points.length === 2 && points[0].x1 !== points[1].x1) {
		// Ensure that point 1 is on the left and point 2 is on the right, so that our
		// subsequent computations don't have to account for that.
		if (points[0].x1 > points[1].x1) {
			let tmp = points[0];
			points[0] = points[1];
			points[1] = tmp;
		}
		zoomAtPinchStart = props.context.zoomLevel;
		txAtPinchStart = SeekBarTransform.fromZoomLevelAndRightEdge(props.context.zoomLevel, props.context.panTimeEndMS, pxToCanvas(canv.clientWidth));
		//rightEdgeMSAtPinchStart = props.context.panTimeEndMS;
		//let t1 = txAtPinchStart.pixelToTime(points[0].x1);
		//let t2 = txAtPinchStart.pixelToTime(points[1].x1);
		//console.log("start", zoomAtPinchStart, txAtPinchStart.canvasWidth, txAtPinchStart.pixelsPerSecond);
		//console.log(new Date(t1));
		//console.log(new Date(t2));
	}
}

function onPointerUp(e: PointerEvent) {
	//console.log("onPointerUp", e.pointerId);
	stopZoom(e);
}

// pointer cancel happens when the user pans (aka scrolls) up/down.
// At first you get the pointer down event, and then as soon as the browser
// decides that this looks like a vertical scroll, it cancels the pointer
// event and takes over.
// We allow pan-y via css "touch-action: pan-y"
function onPointerCancel(e: PointerEvent) {
	//console.log("pointer cancel", e.pointerId);
	stopZoom(e);
}

function stopZoom(e: PointerEvent) {
	//console.log("zoomAtPinchEnd", props.context.zoomLevel, "rightEdge", new Date(props.context.panTimeEndMS));
	let grab = grabber.value! as HTMLDivElement;
	grab.releasePointerCapture(e.pointerId);
	points = points.filter(p => p.id !== e.pointerId);
}

function onPointerMove(e: PointerEvent) {
	//e.preventDefault();
	//console.log("pointer move", e.pointerId);
	if (points.length !== 2) {
		return;
	}
	for (let p of points) {
		if (p.id === e.pointerId) {
			p.x2 = pxToCanvas(e.offsetX);
			p.y2 = pxToCanvas(e.offsetY);
		}
	}
	//let orgDistance = Math.hypot(points[0].x1 - points[1].x1, points[0].y1 - points[1].y1);
	//let newDistance = Math.hypot(points[0].x2 - points[1].x2, points[0].y2 - points[1].y2);

	// We need to solve two things:
	// 1. The new zoom level
	// 2. The new endTime

	// Lock the time of the two finger points, but move their pixel positions, and then
	// solve for the zoom and offset.
	let orgTime1MS = txAtPinchStart.pixelToTime(points[0].x1);
	let orgTime2MS = txAtPinchStart.pixelToTime(points[1].x1);
	let newPixelsPerSecond = (points[1].x2 - points[0].x2) / ((orgTime2MS - orgTime1MS) / 1000);
	//newPixelsPerSecond = txAtPinchStart.pixelsPerSecond;
	props.context.zoomLevel = SeekBarTransform.pixelsPerSecondToZoomLevel(newPixelsPerSecond);

	//console.log(new Date(orgTime1MS));

	let pixelsToRightEdge = txAtPinchStart.canvasWidth - points[1].x2;
	let timeAtRightEdgeMS = orgTime2MS + (pixelsToRightEdge / newPixelsPerSecond) * 1000;
	//timeAtRightEdgeMS = orgTime2MS;
	//console.log(pixelsToRightEdge, newPixelsPerSecond, (timeAtRightEdgeMS - rightEdgeMSAtPinchStart) / 1000);
	props.context.panToMillisecond(timeAtRightEdgeMS);

	//console.log(orgTime1MS / 1000, orgTime2MS / 1000, newPixelsPerSecond, props.context.zoomLevel);

	//console.log("relative scale", newDistance / orgDistance);
	//props.context.zoomLevel = zoomAtPinchStart - Math.log2(newDistance / orgDistance);
	props.context.render(canvas.value! as HTMLCanvasElement);
}

onMounted(() => {
	let canv = canvas.value! as HTMLCanvasElement
	props.context.render(canv);
	poll();
});

</script>

<template>
	<div class="seekBarB">
		<div ref="grabber" class="grabber" @wheel="onWheel" @pointerdown="onPointerDown" @pointerup="onPointerUp"
			@pointermove="onPointerMove" @pointercancel="onPointerCancel" />
		<canvas ref="canvas" class="canvas" />
	</div>
</template>

<style lang="scss" scoped>
@import '@/assets/vars.scss';

.seekBarB {
	//position: relative;
}

.canvas {
	position: absolute;
	width: 100%;
	height: 100%;
	box-sizing: border-box;
	border: solid 1px #000;
	border-top-width: 0;
	background-color: #111;
	touch-action: none;
	border-bottom-left-radius: 5px;
	border-bottom-right-radius: 5px;
}

// Grabber is larger than the canvas, because it's hard to get your thumbs precisely
// inside the canvas area.
.grabber {
	position: absolute;
	left: 0;
	width: 100%;

	// uncomment this line to see the bounds of the grabber. It should be symmetrically bordered around the canvas
	//border: solid 1px #e00;

	// desktop
	// Mouse has much greater precision than thumbs, so we make the padding smaller here.
	top: -5px;
	height: calc(100% + 10px);

	// mobile
	// big margins for fat fingers
	@media (max-width: $mobileCutoff) {
		top: -30px;
		height: calc(100% + 60px);
	}

	z-index: 1;
	touch-action: pan-y; // we want the browser to implement vertical panning, but we want to control pinch-zoom and horizontal panning
}
</style>
