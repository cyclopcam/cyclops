<script setup lang="ts">
import type { Recording } from '@/recording/recording.js';

let props = defineProps<{
	recording?: Recording, // If null, then this is a "record new" pane
}>()
let emits = defineEmits(['click']);

</script>

<template>
	<div :class="{ recording: true, newOuter: !recording, shadow5Hover: !recording }" @click="$emit('click')">
		<img v-if="recording" :src="'/api/record/thumbnail/' + recording.id" class="shadow5" loading="lazy" />
		<div v-else class="flexCenter new">
			New Recording
		</div>
	</div>
</template>

<style lang="scss" scoped>
@import '@/assets/vars.scss';

.recording {
	position: relative;
	width: 200px;
	height: 150px;

	@media (max-width: $mobileCutoff) {
		// fit two per row
		width: 160px;
		height: 120px;
	}
}

img {
	width: 100%;
	height: 100%;
	border-radius: 3px;
}

.newOuter {
	@include flexCenter();
	cursor: pointer;
	border-radius: 3px;
}

.new {
	border: dashed 2px #ddd;
	border-radius: 10px;
	padding: 6px 12px;
}
</style>
