<script setup lang="ts">
import type { CameraRecord } from '@/db/config/configdb';

let props = defineProps<{
	camera: CameraRecord,
}>();
defineEmits(['add']);

</script>

<template>
	<div class="flexRowCenter cameraLine" style="justify-content: space-between">
		<div class="flexRowBaseline">
			<div style="margin: 0 10px">{{ camera.host }}</div>
			<div style="margin: 0 10px">{{ camera.name }}</div>
		</div>
	</div>
</template>

<style lang="scss" scoped>
@import '@/assets/vars.scss';

.cameraLine {
	background-color: $tint;
	padding: 5px;
	border-radius: 10px;
	border: solid 1px $tintBold;
}
</style>
