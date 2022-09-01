<script setup lang="ts">
import type { Recording } from '@/recording/recording';
import Modal from '../../core/Modal.vue';
import Labeler from './Labeler.vue';
import SvgButton from '../../core/SvgButton.vue';
import Left from '@/icons/arrow-big-left.svg';
import Right from '@/icons/arrow-big-right.svg';

let props = defineProps<{
	initialRecording: Recording
}>()
let emits = defineEmits(['close']);
</script>

<template>
	<modal tint="extradark" @close="$emit('close')">
		<div class="outer">
			<!--
			<div v-if="false" class="sideButton">
				<svg-button :icon="Left" />
			</div>
			-->
			<labeler :recording="initialRecording" />
			<!--
			<div v-if="false" class="sideButton">
				<svg-button :icon="Right" />
			</div>
			-->
		</div>
	</modal>
</template>

<style lang="scss" scoped>
.outer {
	background-color: #fff;
	display: flex;
	justify-content: space-between;
	align-items: center;
}

.sideButton {
	padding: 0 20px;
}
</style>
