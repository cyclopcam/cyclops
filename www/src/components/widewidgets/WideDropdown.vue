<script setup lang="ts">
import SelectorIcon from '@/icons/selector.svg';
import Modal from '@/components/widgets/Modal.vue';
import { ref } from 'vue';

let props = defineProps<{
	label: string,
	modelValue: string | null,
	options: string[]
}>()
let emit = defineEmits(['update:modelValue']);

let showModal = ref(false)

function onSelect(value: any) {
	showModal.value = false;
	emit('update:modelValue', value);
}

</script>

<template>
	<div class="widewidget widedropdown">
		<div class="widelabelTop">
			{{ label }}
		</div>
		<div class="valueContainer" @click="showModal = true">
			<div class="value">
				{{ props.modelValue }}
			</div>
			<img :src="SelectorIcon" class="selector" />
		</div>
		<modal v-if="showModal" @close="showModal = false" tint="dark" :scrollable="true">
			<div class="modalContainer">
				<div v-for="opt in options" class="modalElement" @click="onSelect(opt)">
					{{ opt }}
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