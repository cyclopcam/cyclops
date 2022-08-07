<script setup lang="ts">
import type { Recording } from '@/recording/recording.js';

let props = defineProps<{
	recording: Recording
}>()
</script>

<template>
	<div class="recording shadow7L">
		<img :src="'/api/record/thumbnail/' + recording.id" />
	</div>
</template>

<style lang="scss" scoped>
.recording {
	position: relative;
	width: 200px;
	height: 140px;
}

img {
	width: 100%;
	height: 100%;
}
</style>
