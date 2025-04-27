<script setup lang="ts">
import type { CameraInfo } from '@/camera/camera';
import { globals } from '@/globals';
import Player from '@/components/camera/Player.vue';
import { onMounted, onUnmounted, ref } from 'vue';

let isPlaying = ref({} as { [index: number]: boolean }); // ID -> boolean
let linkedPlay = false;
let cameraWidth = ref(320); // Recomputed dynamically

function cameras(): CameraInfo[] {
	return globals.cameras;
}

function onPlayPause(cam: CameraInfo) {
	let newVal = !isPlaying.value[cam.id];
	console.log(`Monitor.vue onPlayPause camera ${cam.id}. newVal = ${newVal}`);
	if (linkedPlay) {
		for (let c of cameras()) {
			isPlaying.value[c.id] = newVal
		}
	} else {
		isPlaying.value[cam.id] = newVal;
	}
}

function onSeek(cam: CameraInfo) {
	//console.log(`Monitor.vue onSeek camera ${cam.id}`);
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
		cameraWidth.value = ww - 8;
	} else {
		// wide screen - could be desktop/ipad/etc
		cameraWidth.value = 360;
	}
}

function cameraHeight(): string {
	// We want an aspect ratio that is the most average, because in <player> we distort the aspect ratio
	// We use aspect = 1.5 because it's more square than 16:9 (1.777), to accomodate cameras that are more square.
	// BUT.. then we lower it even further to 1.4, to make space for the SeekBar
	return `${Math.round(cameraWidth.value / 1.4)}px`;
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
		<div v-if="cameras.length == 0" class="noCameras">
			No cameras configured
		</div>
		<div class="cameras">
			<player v-for="cam of cameras()" :camera="cam" :play="isPlaying[cam.id] ?? false"
				@playpause="onPlayPause(cam)" @seek="onSeek(cam)" :width="cameraWidth + 'px'" :height="cameraHeight()"
				:round="true" />
		</div>
	</div>
</template>

<style lang="scss" scoped>
@import '@/assets/vars.scss';

.monitor {
	align-items: center;
	margin: 0;
	width: 100%;
	height: 100%;
	box-sizing: border-box;
	background-color: $darkBackground1;
	overflow: auto;
}

.noCameras {
	width: 100%;
	height: 100%;
	display: flex;
	justify-content: center;
	align-items: center;
	font-size: 25px;
	font-weight: bold;
	color: $tint;
}

.cameras {
	// We want extra padding on the bottom so that the user knows that this is the last
	// camera, and also so that the user has thumb space to manipulate the seek bar
	// on the last camera.
	padding: 10px 0px 0px 0px;

	// Parent has the background
	//background-color: $darkBackground1;

	display: flex;
	flex-wrap: wrap;
	// This is necessary for centering vertically, but unfortunately it does
	// look a bit weird when you have an odd number of items on the final bin.
	justify-content: center;

	// I need to learn grid properly.. this is just hacking it
	//max-width: 90%;
	//display: grid;
	//grid-template-columns: repeat(auto-fill, 320px);

	gap: 12px;
}
</style>
