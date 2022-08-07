<script setup lang="ts">
import type { CameraInfo } from '@/camera/camera.js';
import { globals } from '@/globals.js';
import CameraItem from '../components/home/CameraItem.vue';

function cameras(): CameraInfo[] {
	return globals.cameras;
}
</script>

<template>
	<div>
		<div>topbar?</div>
		<div class="flexColumn">
			<camera-item v-for="cam of cameras()" :camera="cam" />
		</div>
	</div>
</template>

<style lang="scss" scoped>
</style>
