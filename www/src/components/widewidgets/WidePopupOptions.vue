<script setup lang="ts">
import Modal from '@/components/widgets/Modal.vue';

let props = defineProps<{
	title: string,
	value: string | null,
	options: any[] // Can be an array of strings, or array of objects of type {value: string, label: string}
}>()
let emit = defineEmits(['select', 'cancel']);

function onSelect(opt: any) {
	emit('select', getValue(opt));
}

function getValue(opt: any) {
	return typeof opt === 'object' ? opt.value : opt;
}

function getLabel(opt: any) {
	return typeof opt === 'object' ? opt.label : opt;
}

</script>

<template>
	<modal @close="emit('cancel')" tint="dark" :scrollable="true">
		<div class="modalContainer">
			<div v-for="opt in options" :class="{ option: true, current: getValue(opt) === value }"
				@click="onSelect(opt)">
				{{ getLabel(opt) }}
			</div>
		</div>
	</modal>
</template>

<style lang="scss" scoped>
@import '@/assets/vars.scss';
@import './widewidget.scss';

.modalContainer {
	background: #fff;
	width: 90vw;
	border-radius: 5px;
}

.option {
	padding: 12px 20px;
	border-bottom: 1px solid #eee;
	border-radius: 5px;
	cursor: pointer;
}

.option:first-child {
	padding-top: 15px;
}

.option:last-child {
	padding-bottom: 15px;
}

.current {
	font-weight: 600;
}
</style>