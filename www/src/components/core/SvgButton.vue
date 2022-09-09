<script setup lang="ts">

let props = defineProps<{
	icon: string, // svg drawn on the left of the text
	size?: string, // override default
	invert?: boolean,
	shadow?: boolean,
	disabled?: boolean,
}>()
defineEmits(['click']);

function iconStyle(): any {
	let filter = '';
	if (props.invert) {
		filter += " invert(1) ";
	}
	if (props.shadow) {
		filter += " drop-shadow(0px 0px 2px rgba(0,0,0,0.9)) ";
	}
	if (props.disabled) {
		filter += " contrast(0.05) brightness(1.2) ";
	}

	return {
		width: props.size ?? "",
		height: props.size ?? "",
		filter: filter,
	}
}

</script>

<template>
	<button class="svgButton" @click="$emit('click')">
		<img :src="icon" class="icon" :style="iconStyle()" />
	</button>
</template>

<style lang="scss" scoped>
.svgButton {
	position: relative;
	display: flex;
	align-items: center;
	padding: 4px;
	margin: 0px;
	background: none;
	border: none;
}

.svgButton:hover {
	background: none;
	border: none;
	padding: 3px 5px 5px 3px; // use adjusted padding to simulate button raise
}

.svgButton:active {
	box-shadow: none;
	padding: 4px;
}

.icon {
	width: 20px;
	height: 20px;
}
</style>
