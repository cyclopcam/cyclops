<script setup lang="ts">
import WideRoot from '@/components/widewidgets/WideRoot.vue';
import WideSection from '@/components/widewidgets/WideSection.vue';
import { CameraRecord } from '@/db/config/configdb';
import { fetchOrErr } from '@/util/util';
import { onMounted, ref } from 'vue';
import CameraPreview from './CameraPreview.vue';
import { DetectionZone } from '@/db/config/detectionZone';

let props = defineProps<{
	id: string, // camera id
}>()

let camera = ref(new CameraRecord());
let canvas = ref(null);
let xscale = 1;
let yscale = 1;
let lastPaintEvent: PointerEvent | null = null;

function initDetectionZoneBitmap() {
	let cam = camera.value;
	if (!cam.detectionZone || cam.detectionZone.width === 0) {
		cam.detectionZone = new DetectionZone(60, 40);
		cam.detectionZone.set(59, 39, true);
	}
}

function renderCanvas() {
	let can = canvas.value! as HTMLCanvasElement;
	let dpr = window.devicePixelRatio;
	let width = can.clientWidth * dpr;
	let height = can.clientHeight * dpr;
	can.width = width;
	can.height = height;
	let cam = camera.value;
	let dz = cam.detectionZone;
	if (!dz || dz.width === 0)
		return;
	let cx = can.getContext("2d")!;
	xscale = width / dz.width;
	yscale = height / dz.height;
	for (let y = 0; y < dz.height; y++) {
		for (let x = 0; x < dz.width; x++) {
			if (dz.get(x, y)) {
				cx.fillStyle = "rgba(255, 0, 0, 0.5)";
				cx.fillRect(x * xscale, y * yscale, xscale, yscale);
			}
		}
	}
}

function screenToDz(x: number, y: number): [number, number] {
	let can = canvas.value! as HTMLCanvasElement;
	let dz = camera.value.detectionZone!;
	let dx = Math.floor(x * window.devicePixelRatio / xscale);
	let dy = Math.floor(y * window.devicePixelRatio / yscale);
	return [dx, dy];
}

function onPointerDown(e: PointerEvent) {
	lastPaintEvent = e;
	paint(e);
}

function onPointerUp(e: PointerEvent) {
	lastPaintEvent = null;
}

function onPointerMove(e: PointerEvent) {
	if (e.buttons === 1) {
		if (lastPaintEvent) {
			// Smooth drawing, to avoid gaps caused by sporadic pointer events
			let dx = e.offsetX - lastPaintEvent.offsetX;
			let dy = e.offsetY - lastPaintEvent.offsetY;
			let dist = Math.sqrt(dx * dx + dy * dy);
			if (dist > 1) {
				let steps = Math.floor(dist);
				let stepx = dx / steps;
				let stepy = dy / steps;
				for (let i = 0; i < steps; i++) {
					let x = lastPaintEvent.offsetX + stepx * i;
					let y = lastPaintEvent.offsetY + stepy * i;
					paint(new PointerEvent("pointermove", { clientX: x, clientY: y }), false);
				}
			}
		}
		paint(e);
		lastPaintEvent = e;
	}
}

function paint(e: PointerEvent, render = true) {
	let [tx, ty] = screenToDz(e.offsetX, e.offsetY);
	let dz = camera.value.detectionZone!;
	let brushRadius = 2;
	for (let dx = -brushRadius; dx <= brushRadius; dx++) {
		for (let dy = -brushRadius; dy <= brushRadius; dy++) {
			let x = tx + dx;
			let y = ty + dy;
			if (x >= 0 && x < dz.width && y >= 0 && y < dz.height) {
				dz.set(x, y, true);
			}
		}
	}
	if (render)
		renderCanvas();
}

onMounted(async () => {
	let r = await fetchOrErr(`/api/config/camera/${props.id}`);
	if (r.ok) {
		camera.value = CameraRecord.fromJSON(await r.r.json());
		initDetectionZoneBitmap();
	}
})
</script>

<template>
	<wide-root title="Detection Zone">
		<wide-section>
			<div class="detectionZoneImage">
				<camera-preview :camera="camera" class="preview" />
				<canvas ref="canvas" class="canvas" @pointerdown="onPointerDown" @pointerup="onPointerUp"
					@pointermove="onPointerMove" />
			</div>
		</wide-section>
	</wide-root>
</template>

<style lang="scss" scoped>
@import '@/assets/vars.scss';
@import '@/components/widewidgets/widewidget.scss';

.detectionZoneImage {
	position: relative;
	width: 100%;
	aspect-ratio: 1.5;
}

.preview {
	position: absolute;
	width: 100%;
	height: 100%;
}

.canvas {
	//background-color: rgba(200, 0, 180, 0.1);
	position: absolute;
	width: 100%;
	height: 100%;
	touch-action: none;
}
</style>
