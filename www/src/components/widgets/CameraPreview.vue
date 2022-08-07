<script setup lang="ts">
import type { CameraInfo } from '@/camera/camera.js';

defineProps<{
	camera: CameraInfo,
}>()
</script>

<template>
	<img :src="`/api/camera/latestImage/${camera.id}`" />
</template>

<style lang="scss" scoped>
</style>
