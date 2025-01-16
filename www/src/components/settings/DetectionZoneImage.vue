<script setup lang="ts">
import type { CameraRecord } from '@/db/config/configdb';
import CameraPreview from './CameraPreview.vue';

let props = defineProps<{
	camera: CameraRecord,
}>()
</script>

<template>
	<div class="detectionZoneImage">
		<camera-preview :camera="props.camera" class="preview" />
		<div class="dummy" />
	</div>
</template>

<style lang="scss" scoped>
.detectionZoneImage {
	width: 240px;
	height: 180px;
	position: relative;
}

.preview {
	position: absolute;
	width: 100%;
	height: 100%;
}

.dummy {
	background-color: rgba(200, 0, 180, 0.3);
	position: absolute;
	width: 40px;
	height: 60px;
}
</style>
