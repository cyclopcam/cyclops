<script setup lang="ts">
import type { CameraRecord } from '@/db/config/configdb';
import { randomString } from '@/util/util';
import { nextTick, onMounted, ref, watch } from 'vue';

// CameraPreview shows a recent image from the camera, or it can also
// show an image blob. The image blog path was created for after
// testing a camera. CameraTester returns an image blob from the
// websocket.

// You can set either camera or imageBlob.
// If you set both, then imageBlob takes preference.
let props = defineProps<{
	camera?: CameraRecord,
	imageBlob?: Blob | null,
}>()

enum State {
	Loading,
	Success,
	Failed,
}

let canvas = ref(null);
let imageElement: HTMLImageElement | null = null; // Loaded from props.camera
let imageBitmap: ImageBitmap | null = null; // Loaded from props.imageBlob
let state = ref(State.Loading);

watch(() => props.camera, () => {
	loadImage();
});
watch(() => props.imageBlob, () => {
	loadImage();
});

function drawImage() {
	let can = canvas.value! as HTMLCanvasElement;
	if (!can)
		return;
	if (imageElement) {
		//console.log("CameraPreview rendering HTMLImageElement");
		can.width = imageElement.width;
		can.height = imageElement.height;
		let ctx = can.getContext("2d")!;
		ctx.drawImage(imageElement, 0, 0, can.width, can.height);
	} else if (imageBitmap) {
		//console.log("CameraPreview rendering image blob");
		can.width = imageBitmap.width;
		can.height = imageBitmap.height;
		let ctx = can.getContext("2d")!;
		ctx.drawImage(imageBitmap, 0, 0, can.width, can.height);
	}
}

async function loadImage() {
	state.value = State.Loading;
	if (props.imageBlob) {
		imageElement = null;
		imageBitmap = await createImageBitmap(props.imageBlob);
		state.value = State.Success;
		// wait for nextTick, so that <canvas> is alive
		nextTick(() => {
			drawImage();
		});
	} else if (props.camera && props.camera.id !== 0) {
		imageBitmap = null;
		imageElement = new Image();
		imageElement.onload = () => {
			state.value = State.Success;
			// wait for nextTick, so that <canvas> is alive
			nextTick(() => {
				drawImage();
			});
		}
		imageElement.onerror = (err: any) => {
			console.log(`Failed to load preview image: ${err}`);
			imageElement = null;
			state.value = State.Failed;
		}
		imageElement.src = props.camera.posterURL(randomString(8));
	}
}

onMounted(() => {
	//console.log("CameraPreview Mounted");
	loadImage();
});

</script>

<template>
	<div class="cameraPreview">
		<canvas v-if="state === State.Success" ref="canvas" class="canvas shadow5L" />
		<div v-else-if="state === State.Loading" class="text shadow5L">
			Loading...
		</div>
		<div v-else class="text shadow5L">
			Error!
		</div>
	</div>
</template>

<style lang="scss" scoped>
.cameraPreview {
	width: 280px;
	height: 200px;
}

.canvas {
	width: 100%;
	height: 100%;
	border-radius: 3px;
}

.text {
	display: flex;
	justify-content: center;
	align-items: center;
}
</style>
