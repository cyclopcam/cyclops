<script setup lang="ts">
import Modal from '@/components/widgets/Modal.vue';

let props = defineProps<{
	text?: string
}>()
let emit = defineEmits(['close']);
</script>

<template>
	<modal @close="emit('close')" tint="dark" :scrollable="true">
		<div class="container">
			<div v-if="text !== undefined" class="text">{{ props.text }}</div>
			<slot />
		</div>
	</modal>
</template>

<style lang="scss" scoped>
@import '@/assets/vars.scss';
@import './widewidget.scss';

.container {
	box-sizing: border-box;
	background: #fff;
	max-width: 90vw;
	border-radius: 5px;
	padding: 30px 30px;

	@media (min-width: $mobileCutoff) {
		width: $mobileCutoff;
	}
}

.text {
	line-height: 1.3;
}
</style>