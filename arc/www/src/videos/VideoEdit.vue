<script setup lang="ts">
import { VideoResolution, type Video } from './video';
let props = defineProps<{
	video: Video
}>()

</script>

<template>
	<div class="modalFrame">
		<div>
			<video class="video" controls :src="video.videoUrl(VideoResolution.High)" />
		</div>
		<div class="flexColumn">
			<div>Created By: {{ video.createdBy }}</div>
			<div>Created At: {{ video.createdAt }}</div>
			<div>Camera: {{ video.cameraName }}</div>
		</div>
	</div>
</template>

<style scoped lang="scss">
.video {
	max-height: 80vh;
}
</style>
