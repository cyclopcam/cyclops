<script setup lang="ts">
import type { Ontology, Recording } from '@/recording/recording';
import Modal from '../../core/Modal.vue';
import Labeler from './Labeler.vue';

let props = defineProps<{
	initialRecording: Recording,
	latestOntology: Ontology,
}>()
let emits = defineEmits(['close']);
</script>

<template>
	<modal tint="rgba(0,0,0,0.8)" :fullscreen="true" @close="$emit('close')">
		<labeler :recording="initialRecording" :latest-ontology="latestOntology" />
	</modal>
</template>

<style lang="scss" scoped>

</style>
