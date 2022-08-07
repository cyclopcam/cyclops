<script setup lang="ts">
import type { CameraInfo } from '@/camera/camera.js';
import CameraPreview from '../widgets/CameraPreview.vue';

defineProps<{
	camera: CameraInfo,
}>()
</script>

<template>
	<div class="flex">
		<div>Name</div>
		<camera-preview :camera="camera" />
	</div>
</template>

<style lang="scss" scoped>
</style>
