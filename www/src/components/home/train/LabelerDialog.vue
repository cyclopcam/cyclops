<script setup lang="ts">
import type { Ontology, Recording } from '@/recording/recording';
import Modal from '../../widgets/Modal.vue';
import Labeler from './Labeler.vue';

let props = defineProps<{
	initialRecording: Recording,
	ontologies: Ontology[],
	latestOntology: Ontology,
}>()
let emits = defineEmits(['close']);
</script>

<template>
	<modal tint="rgba(0,0,0,0.8)" :fullscreen="true" @close="emits('close')">
		<labeler :recording="initialRecording" :ontologies="ontologies" :latest-ontology="latestOntology"
			@close="emits('close')" />
	</modal>
</template>

<style lang="scss" scoped>

</style>
