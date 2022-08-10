<script setup lang="ts">

let props = defineProps<{
	icon: string, // svg drawn on the left of the text
	iconSize?: string, // override default
	invert?: boolean,
	shadow?: boolean,
}>()
defineEmits(['click']);

function cls() {
	return {
		svgButton: true,
	}
}

function iconStyle(): any {
	let filter = '';
	if (props.invert) {
		filter += " invert(1) ";
	}
	if (props.shadow) {
		filter += " drop-shadow(0px 0px 2px rgba(0,0,0,0.9)) ";
	}

	return {
		width: props.iconSize ?? "",
		height: props.iconSize ?? "",
		filter: filter,
	}
}

</script>

<template>
	<button :class="cls()" @click="$emit('click')">
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

.icon {
	width: 20px;
	height: 20px;
}
</style>
