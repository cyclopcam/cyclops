<script setup lang="ts">
import WideRoot from '@/components/widewidgets/WideRoot.vue';
import WideSection from '@/components/widewidgets/WideSection.vue';
import WideSaveCancel from '@/components/widewidgets/WideSaveCancel.vue';
import WideError from '@/components/widewidgets/WideError.vue';
import PaintBlocks from './PaintBlocks.vue';
import { CameraRecord } from '@/db/config/configdb';
import { fetchOrErr } from '@/util/util';
import { onMounted, ref } from 'vue';
import CameraPreview from './CameraPreview.vue';
import { DetectionZone } from '@/db/config/detectionZone';
import { globals } from '@/globals';

let props = defineProps<{
	id: string, // camera id
}>()

let camera = ref(new CameraRecord());
let canvas = ref(null);
let lastPaintEvent: PointerEvent | null = null;
let paintHot = ref(true);
let error = ref("");
let busySaving = ref(false);
let saveStatus = ref("");
let isModified = ref(false);

function initDetectionZoneBitmap() {
	let cam = camera.value;
	if (!cam.detectionZone || cam.detectionZone.width === 0) {
		cam.detectionZone = new DetectionZone(64, 40);
		cam.detectionZone.fill(true);
		paintHot.value = false;
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
	let xscale = width / dz.width;
	let yscale = height / dz.height;
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
	let el = canvas.value;
	if (!el)
		return [0, 0];
	let can = el as HTMLCanvasElement;
	let width = can.clientWidth;
	let height = can.clientHeight;
	let dx = Math.floor(x / width * camera.value.detectionZone!.width);
	let dy = Math.floor(y / height * camera.value.detectionZone!.height);
	return [dx, dy];
}

function onPointerDown(e: PointerEvent) {
	lastPaintEvent = e;
	paint(e.offsetX, e.offsetY, true);
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
					paint(x, y, false);
				}
			}
		}
		paint(e.offsetX, e.offsetY, true);
		lastPaintEvent = e;
	}
}

function paint(offsetX: number, offsetY: number, render = true) {
	let [tx, ty] = screenToDz(offsetX, offsetY);
	let dz = camera.value.detectionZone!;
	let brushRadius = 3;
	let hot = paintHot.value;
	isModified.value = true;
	for (let dx = -brushRadius; dx <= brushRadius; dx++) {
		for (let dy = -brushRadius; dy <= brushRadius; dy++) {
			let distance = Math.hypot(dx, dy);
			if (distance + 0.5 > brushRadius)
				continue;
			let x = tx + dx;
			let y = ty + dy;
			if (x >= 0 && x < dz.width && y >= 0 && y < dz.height) {
				dz.set(x, y, hot);
			}
		}
	}
	if (render)
		renderCanvas();
}

async function onSave() {
	busySaving.value = true;
	saveStatus.value = "Saving...";
	let r = await camera.value.saveSettingsToServer();
	busySaving.value = false;
	if (!r.ok) {
		saveStatus.value = "";
		error.value = r.error;
		return;
	} else {
		saveStatus.value = "Saved";
		isModified.value = false;
		setTimeout(() => saveStatus.value = "", 1000);
	}
}

onMounted(async () => {
	let r = await fetchOrErr(`/api/config/camera/${props.id}`);
	if (r.ok) {
		camera.value = CameraRecord.fromJSON(await r.r.json());
		initDetectionZoneBitmap();
		renderCanvas();
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
			<div :class="{ 'paintButton': true, 'activeButton': paintHot }" @click="paintHot = true">
				<paint-blocks :hot-zone="true" size="20px" style="margin-right: 12px" />
				Detection Zone
			</div>
			<div :class="{ 'paintButton': true, 'activeButton': !paintHot }" @click="paintHot = false">
				<paint-blocks :hot-zone="false" size="20px" style="margin-right: 12px" />
				Erase
			</div>
			<div class="explain">Objects must enter the red region to trigger the alarm.</div>
			<wide-error v-if="error">{{ error }}</wide-error>
			<wide-save-cancel :can-save="!busySaving && isModified" :status="saveStatus" @save="onSave" />
		</wide-section>
	</wide-root>
</template>

<style lang="scss" scoped>
@import '@/assets/vars.scss';
@import '@/components/widewidgets/widewidget.scss';

.detectionZoneImage {
	position: relative;
	//width: calc(100% - 10px);
	aspect-ratio: 1.5;
	margin: 26px 10px;
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

.paintButton {
	display: flex;
	margin: 4px 10px 0px 10px;
	border-radius: 8px;
	padding: 6px 6px;
	border: solid 3px rgba(0, 0, 0, 0);
}

.activeButton {
	background: #f8f8f8;
	border: solid 3px #44d
}

.explain {
	margin: 15px 20px 15px 20px;
}
</style>
