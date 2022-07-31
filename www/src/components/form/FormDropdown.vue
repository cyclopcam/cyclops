<script setup lang="ts">
import type * as forms from './forms';
import Modal from '../core/Modal.vue';
import { ref } from 'vue';

let props = defineProps<{
	ctx: forms.Context,
	modelValue: string,
	options: string[],
	label?: string,
	required?: boolean,
	placeholder?: string,
}>()
let emits = defineEmits(['update:modelValue']);

let showDrop = ref(false);

function rollupStyle(): any {
	return {
		"width": props.ctx.inputWidth.value,
	};
}

function onPick(option: string) {
	emits('update:modelValue', option);
	showDrop.value = false;
}

</script>

<template>
	<div class="formItem">
		<slot name="label">
			<div v-if="label" class="label">
				{{ label }}
			</div>
		</slot>
		<div @click="showDrop = true" class="rollup" :style="rollupStyle()">
			{{ modelValue }}
			<div class="rollupDown"></div>
		</div>
		<modal v-if="showDrop" @close="showDrop = false" position="previous" relative="under" :same-width="true">
			<div class="dropContainer shadow15">
				<div v-for="opt of options" class="option" @click="onPick(opt)">{{ opt }}</div>
			</div>
		</modal>
	</div>
</template>

<style lang="scss" scoped>
@import '@/assets/vars.scss';
@import '@/components/form/forms.scss';

$rad: 5px;

.rollup {
	user-select: none;
	cursor: pointer;
	display: flex;
	justify-content: space-between;
	border-bottom: $formBorderBottom;
	font-size: $formInputFontSize;
}

.rollupDown {
	width: 16px;
	height: 16px;
	background-image: url('@/icons/chevron-down.svg');
	background-size: 16px 16px;
}

.dropContainer {
	user-select: none;
	border-radius: $rad;
}

.option {
	background-color: #fff;
	padding: 6px 12px;

	@media (max-width: $mobileCutoff) {
		padding: 12px 14px;
	}
}

.option:first-child {
	padding-top: 12px;

	@media (max-width: $mobileCutoff) {
		padding-top: 16px;
	}

	border-top-left-radius: $rad;
	border-top-right-radius: $rad;
}

.option:last-child {
	padding-bottom: 12px;

	@media (max-width: $mobileCutoff) {
		padding-bottom: 16px;
	}

	border-bottom-left-radius: $rad;
	border-bottom-right-radius: $rad;
}

.option:hover {
	background-color: #eee;
}
</style>
