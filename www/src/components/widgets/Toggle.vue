<script setup lang="ts">
//import { defineProps, defineEmits } from 'vue';

const props = defineProps<{ modelValue: boolean }>();
const emit = defineEmits(['update:modelValue', 'change']);

const toggle = () => {
	//console.log('toggle', !props.modelValue);
	emit('update:modelValue', !props.modelValue);
	emit('change', !props.modelValue);
};
</script>

<template>
	<div class="toggle" :class="{ 'on': modelValue }" @click="toggle">
		<div class="inner"></div>
	</div>
</template>

<style scoped>
.toggle {
	width: 46px;
	height: 27px;
	border-radius: 27px;
	background-color: #ccc;
	position: relative;
	cursor: pointer;
	transition: background-color 0.1s;
	-webkit-tap-highlight-color: transparent;
	touch-action: manipulation;
	box-shadow: inset 1px 1px 3px 0px rgba(0, 0, 0, 0.15);
}

.toggle.on {
	background-color: #3553a5;
}

.inner {
	width: 23px;
	height: 23px;
	background-color: #fff;
	border-radius: 50%;
	position: absolute;
	top: 2px;
	left: 2px;
	transition: left 0.1s;
}

.toggle.on .inner {
	left: 21px;
}
</style>