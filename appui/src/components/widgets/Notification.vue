<script setup lang="ts">
import Modal from "./Modal.vue";

let props = defineProps<{
	text: string,
}>()

</script>

<template>
	<modal tint="light">
		<div class="notification">
			{{ text }}
		</div>
	</modal>
</template>

<style lang="scss" scoped>
.notification {}
</style>
