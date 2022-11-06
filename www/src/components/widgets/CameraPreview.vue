<script setup lang="ts">
import type { CameraInfo } from '@/camera/camera';
import { encodeQuery } from '@/util/util';

let props = defineProps<{
	camera: CameraInfo,
}>()

function cameraURL(): string {
	//return `/api/camera/latestImage/${props.camera.id}?` + encodeQuery(bearerTokenQuery());
	return `/api/camera/latestImage/${props.camera.id}`;
}
</script>

<template>
	<div>
		<img class="preview" :src="cameraURL()" />
	</div>
</template>

<style lang="scss" scoped>
.preview {
	border-radius: 5px;
}
</style>
