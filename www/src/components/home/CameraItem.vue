<script setup lang="ts">
import type { CameraInfo } from '@/camera/camera.js';
import CameraPreview from '../widgets/CameraPreview.vue';
import Player from '../camera/Player.vue';

defineProps<{
	camera: CameraInfo,
	play: boolean
}>()
defineEmits(['play', 'stop']);
</script>

<template>
	<div class="flex cameraItem">
		<!-- <camera-preview :camera="camera" /> -->
		<player :camera="camera" :play="play" @click="$emit('stop')" />
		<div v-if="!play" class="play" @click="$emit('play')"></div>
	</div>
</template>

<style lang="scss" scoped>
.cameraItem {
	position: relative;
}

.play {
	position: absolute;
	left: 0px;
	top: 0px;
	background-repeat: no-repeat;
	background-image: url("@/icons/play-circle.svg");
	background-size: 50px 50px;
	background-position: center;
	width: 100%;
	height: 100%;
	filter: invert(1) drop-shadow(1px 1px 3px rgba(0, 0, 0, 0.9));
	cursor: pointer;
}
</style>
