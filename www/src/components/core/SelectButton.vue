<script setup lang="ts">
let props = defineProps<{
	modelValue: boolean | null,
}>()
let emit = defineEmits(['update:modelValue']);

function onClick() {
	emit('update:modelValue', !props.modelValue);
}

</script>

<template>
	<div :class="{ selectButton: true, selected: modelValue }" @click="onClick">
		<div :class="{ inside: true, insidePlaceholder: !modelValue, insideSelected: modelValue }"></div>
	</div>
</template>

<style lang="scss" scoped>
@import '@/assets/vars.scss';

$size: 22px;

.selectButton {
	border: solid 1px #ddd;
	width: $size;
	height: $size;
	cursor: pointer;
}

.selectButton:hover {
	border-color: #88d;
}

.selected {
	background-color: $toggleColor;
	border-color: $toggleColor;
}

.inside {
	width: $size;
	height: $size;
	background-image: url('@/icons/check.svg');
	background-size: $size $size;
	background-repeat: no-repeat;
}

.insidePlaceholder {
	filter: contrast(0.1) brightness(2);
}

.insideSelected {
	filter: invert(1);
}
</style>
