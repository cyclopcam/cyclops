<script setup lang="ts">
import type { CameraInfo } from '@/camera/camera';
import { globals } from '@/globals';
import CameraItem from '@/components/home/CameraItem.vue';
import { ref } from 'vue';

let play = ref(false);

function cameras(): CameraInfo[] {
	return globals.cameras;
}

function onPlay() {
	console.log("onPlay");
	play.value = true;
}

function onStop() {
	console.log("onStop");
	play.value = false;
}

</script>

<template>
	<div class="flexColumn monitor">
		<!--
		<toolbar style="margin: 15px 10px 10px 10px">
			<button>Play</button>
		</toolbar>
		-->
		<div class="cameras">
			<camera-item v-for="cam of cameras()" :camera="cam" :play="play" @play="onPlay" @stop="onStop"
				size="medium" />
		</div>
	</div>
</template>

<style lang="scss" scoped>
.monitor {
	align-items: center;
	margin: 10px 0 0 0;
}

.cameras {
	//background-color: beige;
	padding: 10px 10px;

	display: flex;
	flex-wrap: wrap;
	// This is necessary for centering vertically, but unfortunately it does
	// look a bit weird when you have an odd number of items on the final bin.
	justify-content: center;

	// I need to learn grid properly.. this is just hacking it
	//max-width: 90%;
	//display: grid;
	//grid-template-columns: repeat(auto-fill, 320px);

	gap: 20px;
}
</style>
