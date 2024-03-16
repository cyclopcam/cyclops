<script setup lang="ts">
import SelectorIcon from '@/icons/selector.svg';
import Modal from '@/components/widgets/Modal.vue';
import { ref } from 'vue';

let props = defineProps<{
	label: string,
	modelValue: string | null,
	options: any[] // Can be an array of strings, or array of objects of type {value:string, label: string}
}>()
let emit = defineEmits(['update:modelValue']);

let showModal = ref(false)

function onSelect(value: any) {
	showModal.value = false;
	if (typeof value === 'object')
		emit('update:modelValue', value.value);
	else
		emit('update:modelValue', value);
}

// Given a value, such as "motion", return the label associated with it, such as "Whenever there is motion"
function valueToLabel(value: string | null) {
	if (value == null)
		return "";
	if (typeof props.options[0] === 'object') {
		let obj = props.options.find(o => o.value === value);
		if (obj)
			return obj.label;
		return value;
	}
	return value;
}

function displayValue(value: any) {
	if (typeof value === 'object')
		return value.label;
	return value;
}

</script>

<template>
	<div class="widewidget widedropdown">
		<div class="widelabelTop">
			{{ label }}
		</div>
		<div class="valueContainer" @click="showModal = true">
			<div class="value">
				{{ valueToLabel(props.modelValue) }}
			</div>
			<img :src="SelectorIcon" class="selector" />
		</div>
		<modal v-if="showModal" @close="showModal = false" tint="dark" :scrollable="true">
			<div class="modalContainer">
				<div v-for="opt in options" class="modalElement" @click="onSelect(opt)">
					{{ displayValue(opt) }}
				</div>
			</div>
		</modal>
	</div>
</template>

<style lang="scss" scoped>
@import '@/assets/vars.scss';
@import './widewidget.scss';

.widedropdown {
	display: flex;
	flex-direction: column;
}

.valueContainer {
	width: 100%;
	display: flex;
	align-items: center;
	justify-content: space-between;
}

.value {
	margin-left: $wideLeftMargin;
}

.selector {
	width: 20px;
	height: 20px;
}

.modalContainer {
	background: #fff;
	width: 90vw;
	border-radius: 5px;
}

.modalElement {
	padding: 12px 20px;
	border-bottom: 1px solid #eee;
	border-radius: 5px;
}
</style>