<script setup lang="ts">
import type { CameraRecord } from '@/db/config/configdb';

let props = defineProps<{
	camera: CameraRecord,
	isConfigured: boolean,
}>();
defineEmits(['add']);

</script>

<template>
	<div class="flexRowCenter" style="justify-content: space-between">
		<div class="flexRowBaseline">
			<div style="margin: 0 10px">{{ camera.host }}</div>
			<div style="margin: 0 10px">{{ camera.model }}</div>
		</div>
		<div v-if="isConfigured" style="color: #080">Configured</div>
		<button v-else @click="$emit('add')">Add</button>
	</div>
</template>

<style lang="scss" scoped>
</style>
