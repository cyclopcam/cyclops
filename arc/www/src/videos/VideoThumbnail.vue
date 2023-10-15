<script setup lang="ts">
import type { Video } from './video';

let props = defineProps<{
	video: Video
}>()

</script>

<template>
	<div>
		<img :src="props.video.thumbnailUrl()" />
	</div>
</template>

<style scoped lang="scss"></style>
