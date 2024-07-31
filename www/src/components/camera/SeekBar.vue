<script setup lang="ts">
import type { CameraInfo } from '@/camera/camera';
import { onMounted, onUnmounted, ref } from 'vue';
import { SeekBarContext } from './seekBarContext';
import { globalTileCache } from './eventTileCache';

let props = defineProps<{
	camera: CameraInfo,
	context: SeekBarContext,
}>()

let canvas = ref(null);
let lastRenderFetchCount = 0;

/*
// TEMP!
function poll() {
	if (!canvas.value) {
		return;
	}
	let canv = canvas.value! as HTMLCanvasElement;

	if (globalTileCache.fetchCount != lastRenderFetchCount) {
		let canv = canvas.value! as HTMLCanvasElement
		props.context.render(canv);
	}
	lastRenderFetchCount = globalTileCache.fetchCount;
	setTimeout(poll, 500);
}
*/

function onWheel(e: WheelEvent) {
	props.context.zoomLevel += (e.deltaY / 100) * 0.2;
	props.context.render(canvas.value! as HTMLCanvasElement);
}

onMounted(() => {
	let canv = canvas.value! as HTMLCanvasElement
	props.context.render(canv);
	//poll();
});

</script>

<template>
	<canvas ref="canvas" @wheel="onWheel" />
</template>

<style lang="scss" scoped></style>
