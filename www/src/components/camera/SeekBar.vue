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
	offsetX1: number; // CSS x at finger down
	x1: number; // canvas x at finger down
	y1: number; // canvas y at finger down
	x2: number; // latest canvas x finger
	y2: number; // latest canvas y finger
}

enum States {
	Neutral,
	Seek,
	Zoom,
}

let canvas = ref(null);
let grabber = ref(null);
let points: Point[] = []; // when there are 2 points, points[0] is on the left and points[1] is on the right
let txAtPinchStart = new SeekBarTransform();
let state = States.Neutral;

// Convert from CSS pixel (eg from PointerEvent) to our canvas coordinates, which are native device pixels
function pxToCanvas(cssPx: number): number {
	return cssPx * window.devicePixelRatio;
}

function autoPanToEndAndRender() {
	if (!canvas.value) {
		// canvas has been destroyed
		return;
	}
	if (props.context.panTimeEndIsNow) {
		let canv = canvas.value! as HTMLCanvasElement;
		props.context.panToNow();
		props.context.render(canv);
	}
}

function poll() {
	if (!canvas.value) {
		// end poll - canvas has been destroyed
		return;
	}
	autoPanToEndAndRender();
	setTimeout(poll, 5000);
}

// On desktop, you can scroll with the mouse wheel
function onWheel(e: WheelEvent) {
	//console.log("onWheel", e.deltaX, e.deltaY);
	e.preventDefault();
	let zoomDelta = (e.deltaY / 100) * 0.3;
	zoomAroundSinglePoint(e.offsetX, zoomDelta);
}

// I don't like the double-click, because it pans the seek bar by a sudden jump
// which destroys your sense of where you were.
//function onDblClick(e: MouseEvent) {
//	console.log("onDblClick");
//	let canv = canvas.value! as HTMLCanvasElement;
//	let canvasWidth = canv.clientWidth;
//	let tx = props.context.transform(canv);
//	let panMS = tx.pixelToTime(pxToCanvas(e.offsetX + canvasWidth * 0.25));
//	let seekMS = tx.pixelToTime(pxToCanvas(e.offsetX));
//	props.context.zoomLevel -= 1;
//	props.context.panToMillisecond(panMS);
//	props.context.seekToMillisecond(seekMS);
//	props.context.render(canv);
//}

function onPointerDown(e: PointerEvent) {
	//console.log("onPointerDown", e.pointerId);
	let canv = canvas.value! as HTMLCanvasElement
	let grab = grabber.value! as HTMLDivElement;
	grab.setPointerCapture(e.pointerId);
	let ox = pxToCanvas(e.offsetX);
	let oy = pxToCanvas(e.offsetY);
	points.push({ id: e.pointerId, offsetX1: e.offsetX, x1: ox, x2: ox, y1: oy, y2: oy });
	if (points.length === 2 && points[0].x1 !== points[1].x1) {
		// Ensure that point 1 is on the left and point 2 is on the right, so that our
		// subsequent computations don't have to account for that.
		if (points[0].x1 > points[1].x1) {
			let tmp = points[0];
			points[0] = points[1];
			points[1] = tmp;
		}
		txAtPinchStart = SeekBarTransform.fromZoomLevelAndRightEdge(props.context.zoomLevel, props.context.panTimeEndMS, pxToCanvas(canv.clientWidth));
	}

	// For a mouse, start seeking the moment the mouse goes down.
	if (e.pointerType === "mouse" && points.length === 1) {
		onPointerMove(e);
	}
}

function onPointerUp(e: PointerEvent) {
	//console.log("onPointerUp", e.pointerId);
	pointerUpOrCancel(e);
}

// pointer cancel happens when the user pans (aka scrolls) up/down.
// At first you get the pointer down event, and then as soon as the browser
// decides that this looks like a vertical scroll, it cancels the pointer
// event and takes over.
// We allow pan-y via css "touch-action: pan-y"
function onPointerCancel(e: PointerEvent) {
	//console.log("pointer cancel", e.pointerId);
	pointerUpOrCancel(e);
}

function pointerUpOrCancel(e: PointerEvent) {
	//console.log("pointerUpOrCancel", props.context.zoomLevel, "rightEdge", new Date(props.context.panTimeEndMS));
	let grab = grabber.value! as HTMLDivElement;
	grab.releasePointerCapture(e.pointerId);
	points = points.filter(p => p.id !== e.pointerId);
	if (points.length === 0) {
		// Only reset to Neutral once both fingers lift.
		// This is to prevent a pinch-zoom from becoming a seek after one of the fingers
		// is lifted up, but the other fingers remains for a few milliseconds.
		state = States.Neutral;
	}
}

function onPointerMove(e: PointerEvent) {
	//console.log("pointer move", e.pointerId);
	if (points.length === 1) {
		onPointerMoveSeek(e);
	} else if (points.length === 2) {
		onPointerMovePinchZoom(e);
	}
}

function onPointerMoveSeek(e: PointerEvent) {
	// Don't start a seek until we've made a decently large horizontal swipe.
	// Without this protection, you very often end up seeking the bar when all you
	// wanted to do was scroll the entire monitor screen vertically, to get
	// to another camera.
	let minDelta = 10;
	// We disable this behaviour for a mouse, because a mouse movement can't invoke a vertical scroll.
	if (e.pointerType === "mouse") {
		minDelta = 0;
	}
	let cssDeltaX = Math.abs(e.offsetX - points[0].offsetX1);
	if (state === States.Neutral && cssDeltaX >= minDelta) {
		state = States.Seek;
	}
	if (state !== States.Seek) {
		return;
	}
	let x = pxToCanvas(e.offsetX);
	let tx = props.context.transform(canvas.value! as HTMLCanvasElement);
	let timeMS = tx.pixelToTime(x);
	props.context.seekToMillisecond(timeMS);
	props.context.render(canvas.value! as HTMLCanvasElement);
}

function onPointerMovePinchZoom(e: PointerEvent) {
	state = States.Zoom;

	for (let p of points) {
		if (p.id === e.pointerId) {
			p.x2 = pxToCanvas(e.offsetX);
			p.y2 = pxToCanvas(e.offsetY);
		}
	}

	// We need to solve two things:
	// 1. The new zoom level
	// 2. The new endTime

	// Lock the time of the two finger points, but move their pixel positions, and then
	// solve for the zoom and offset.

	// If you need a mental framework to think about what's going on here:
	// The points in time where the fingers went down remain constant.
	// What's being dragged by the two fingers is the pixel positions of those time points.

	let orgTime1MS = txAtPinchStart.pixelToTime(points[0].x1);
	let orgTime2MS = txAtPinchStart.pixelToTime(points[1].x1);
	let newPixelsPerSecond = (points[1].x2 - points[0].x2) / ((orgTime2MS - orgTime1MS) / 1000);
	props.context.zoomLevel = SeekBarTransform.pixelsPerSecondToZoomLevel(newPixelsPerSecond);

	//console.log(new Date(orgTime1MS));

	let pixelsToRightEdge = txAtPinchStart.canvasWidth - points[1].x2;
	let timeAtRightEdgeMS = orgTime2MS + (pixelsToRightEdge / newPixelsPerSecond) * 1000;
	props.context.panToMillisecond(timeAtRightEdgeMS);

	//console.log(orgTime1MS / 1000, orgTime2MS / 1000, newPixelsPerSecond, props.context.zoomLevel);

	props.context.render(canvas.value! as HTMLCanvasElement);
}

// Zoom around a single point, eg when zooming in/out with the mouse wheel
// The critical thing here is that the pan position of the mouse cursor remains constant.
function zoomAroundSinglePoint(offsetX: number, zoomDelta: number) {
	let x = pxToCanvas(offsetX);
	let canv = canvas.value! as HTMLCanvasElement;
	let txOld = props.context.transform(canv);
	let timeMS = txOld.pixelToTime(x);
	props.context.zoomLevel += zoomDelta;

	let txNew = props.context.transform(canv);
	let pixelsToRightEdge = txOld.canvasWidth - x;
	let msPerPixel = 1000 / txNew.pixelsPerSecond;
	let msToRightEdge = pixelsToRightEdge * msPerPixel;
	let timeAtRightEdgeMS = timeMS + msToRightEdge;
	props.context.panToMillisecond(timeAtRightEdgeMS);

	props.context.render(canv);
}

onMounted(() => {
	let canv = canvas.value! as HTMLCanvasElement
	props.context.render(canv);

	// On mobile there's this behaviour where the initial render has a slightly different
	// scale to the first polled render (which comes 5 seconds after page load). This
	// 100ms timeout is a hack to fix that. I assume we're getting some kind of layout
	// adjustment that all happens before anything is rendered, and that's causing the
	// discrepancy.
	setTimeout(autoPanToEndAndRender, 100);

	// Start our slow poller
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
