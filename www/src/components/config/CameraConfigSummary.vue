<script setup lang="ts">
import type { CameraRecord } from '@/db/config/configdb';

let props = defineProps<{
	camera: CameraRecord
}>();
defineEmits(['add']);

</script>

<template>
	<div class="flexRowCenter" style="justify-content: space-between">
		<div class="flexRowBaseline">
			<div style="margin: 0 10px">{{ camera.host }}</div>
			<div style="margin: 0 10px">{{ camera.model }}</div>
		</div>
		<button @click="$emit('add')">Add</button>
	</div>
</template>

<style lang="scss" scoped>
</style>
