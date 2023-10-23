<script setup lang="ts">
import type { Video } from './video';

let props = defineProps<{
	video: Video
}>()

</script>

<template>
	<div>
		<img :src="props.video.thumbnailUrl()" class="img" />
	</div>
</template>

<style scoped lang="scss">
.img {
	width: 400px;
	height: 280px;
	object-fit: scale-down;
}
</style>
