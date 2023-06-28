<script setup lang="ts">

//let props = defineProps<{
//	//label: string,
//}>()

let emit = defineEmits(['click']);

function onClick() {
	emit('click');
}

</script>

<template>
	<div class="widewidget widebutton" @click="onClick">
		<slot />
	</div>
</template>

<style lang="scss" scoped>
@import '@/assets/vars.scss';
@import './widewidget.scss';

.widebutton {
	display: flex;
	justify-content: center;
	align-items: center;
	user-select: none;
	cursor: pointer;
}
</style>