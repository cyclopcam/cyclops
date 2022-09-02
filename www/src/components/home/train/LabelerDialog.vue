<script setup lang="ts">
import type { Recording } from '@/recording/recording';
import Modal from '../../core/Modal.vue';
import Labeler from './Labeler.vue';

let props = defineProps<{
	initialRecording: Recording
}>()
let emits = defineEmits(['close']);
</script>

<template>
	<modal tint="rgba(0,0,0,0.8)" @close="$emit('close')">
		<labeler :recording="initialRecording" />
	</modal>
</template>

<style lang="scss" scoped>
</style>
