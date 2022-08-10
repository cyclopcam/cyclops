<script setup lang="ts">
import type { Recording } from '@/recording/recording.js';
import { ref } from 'vue';
import Modal from '../../core/Modal.vue';

let props = defineProps<{
	recording?: Recording, // If null, then this is a "record new" pane
}>()
let emits = defineEmits(['click']);

let showPlayer = ref(false);

function onImgClick() {
	showPlayer.value = true;
}

</script>

<template>
	<div :class="{ recording: true, newOuter: !recording, shadow5Hover: !recording }" @click="$emit('click')">
		<img v-if="recording && !showPlayer" :src="'/api/record/thumbnail/' + recording.id" class="shadow5"
			loading="lazy" @click="onImgClick" />

		<div v-if="!recording" class="flexCenter new">
			New Recording
		</div>
		<modal v-if="recording && showPlayer" tint="rgba(255,255,255,0.8)">
			<div class="player">
				<video :src="'/api/record/video/' + recording.id" class="popupVideo" />
			</div>
		</modal>
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

.popupVideo {
	width: 400px;
	border: solid 2px #333;
	border-radius: 3px;
	box-shadow: 5px 5px 15px rgba(0, 0, 0, 0.5);
}
</style>
