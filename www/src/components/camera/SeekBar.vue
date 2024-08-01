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

let canvas = ref(null);

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
	props.context.zoomLevel += (e.deltaY / 100) * 0.3;
	props.context.zoomLevel = clamp(props.context.zoomLevel, 0, MaxTileLevel + 2);
	props.context.render(canvas.value! as HTMLCanvasElement);
}

onMounted(() => {
	let canv = canvas.value! as HTMLCanvasElement
	props.context.render(canv);
	poll();
});

</script>

<template>
	<canvas ref="canvas" class="seekBar" @wheel="onWheel" />
</template>

<style lang="scss" scoped>
.seekBar {
	box-sizing: border-box;
	border: solid 1px #000;
	border-top-width: 0;
	background-color: #111;
}
</style>
