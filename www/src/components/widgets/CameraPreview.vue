<script setup lang="ts">
import type { CameraInfo } from '@/camera/camera';

defineProps<{
	camera: CameraInfo,
}>()
</script>

<template>
	<div>
		<img class="preview" :src="`/api/camera/latestImage/${camera.id}`" />
	</div>
</template>

<style lang="scss" scoped>
.preview {
	border-radius: 5px;
}
</style>
