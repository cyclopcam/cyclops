<script setup lang="ts">
import type { CameraInfo } from '@/camera/camera';
import { globals } from '@/globals';
import CameraItem from '@/components/home/CameraItem.vue';
import { onMounted, onUnmounted, ref } from 'vue';
import { on } from 'events';

let isPlaying = ref({} as { [index: number]: boolean }); // ID -> boolean
let linkedPlay = false;
let cameraWidth = ref(320);

function cameras(): CameraInfo[] {
	return globals.cameras;
}

function onPlay(cam: CameraInfo) {
	console.log(`Monitor onPlay camera ${cam.id}`);
	if (linkedPlay) {
		for (let c of cameras()) {
			isPlaying.value[c.id] = true;
		}
	} else {
		isPlaying.value[cam.id] = true;
	}
}

function onStop(cam: CameraInfo) {
	console.log("onStop");
	if (linkedPlay) {
		for (let c of cameras()) {
			isPlaying.value[c.id] = false;
		}
	} else {
		isPlaying.value[cam.id] = false;
	}
}

function onWindowResize() {
	let ww = window.innerWidth;
	//console.log("resize", ww);
	if (ww < 450) {
		// Cellphone screen in portrait.
		// The largest phone screen in the Chrome debug tools is the iPhone 14 Pro Max, which is 430 pixels wide.
		// The width of the screen is our major constraint here, and we want to maximize the width of the camera view
		// We need *some* margin here, otherwise scrolling your thumb to the right edge is awkward.
		cameraWidth.value = ww - 4;
	} else {
		// wide screen - could be desktop/ipad/etc
		cameraWidth.value = 360;
	}
}

function cameraHeight(): string {
	return `${cameraWidth.value / 1.4}px`;
}

onMounted(() => {
	window.addEventListener('resize', onWindowResize);
	onWindowResize();
});

onUnmounted(() => {
	window.removeEventListener('resize', onWindowResize);
});

</script>

<template>
	<div class="flexColumn monitor">
		<!--
		<toolbar style="margin: 15px 10px 10px 10px">
			<button>Play</button>
		</toolbar>
		-->
		<div class="cameras">
			<camera-item v-for="cam of cameras()" :camera="cam" :play="isPlaying[cam.id] ?? false" @play="onPlay(cam)"
				@stop="onStop(cam)" :width="cameraWidth + 'px'" :height="cameraHeight()" />
		</div>
	</div>
</template>

<style lang="scss" scoped>
.monitor {
	align-items: center;
	margin: 0px 0 0 0;
}

.cameras {
	background-color: #222;
	padding: 5px 0px;

	display: flex;
	flex-wrap: wrap;
	// This is necessary for centering vertically, but unfortunately it does
	// look a bit weird when you have an odd number of items on the final bin.
	justify-content: center;

	// I need to learn grid properly.. this is just hacking it
	//max-width: 90%;
	//display: grid;
	//grid-template-columns: repeat(auto-fill, 320px);

	gap: 10px;
}
</style>
