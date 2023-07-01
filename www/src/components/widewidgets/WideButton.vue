<script setup lang="ts">

let props = defineProps<{
	disabled?: boolean,
	icon?: string,
}>()

let emit = defineEmits(['click']);

function onClick() {
	if (!props.disabled) {
		emit('click');
	}
}

function iconStyle(): any {
	return {}
}

</script>

<template>
	<div :class="{ widewidget: true, widebutton: true, centered: !icon }" @click="onClick">
		<div v-if="icon" class="iconContainer">
			<img :src="icon" class="icon" :style="iconStyle()" />
		</div>
		<slot />
	</div>
</template>

<style lang="scss" scoped>
@import '@/assets/vars.scss';
@import './widewidget.scss';

.widebutton {
	display: flex;
	align-items: center;
	user-select: none;
	cursor: pointer;
	min-height: 56px;
}

.centered {
	justify-content: center;
}

.iconContainer {
	margin-right: 20px;
	display: flex;
	align-items: center;
}
</style>