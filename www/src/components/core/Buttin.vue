<script setup lang="ts">

let props = defineProps<{
	busy?: boolean, // show 'busy' animation. busy implies disabled.
	disabled?: boolean, // No need to set disabled if you already set busy.
	danger?: boolean, // Red
	focal?: boolean, // One focal button per screen
	icon?: string, // svg drawn on the left of the text
	iconSize?: string, // override default
}>()
defineEmits(['click']);

function cls() {
	return {
		buttin: true,
		dangerButton: props.danger,
		focalButton: props.focal,
	}
}

function iconStyle(): any {
	return {
		width: props.iconSize ?? "",
		height: props.iconSize ?? "",
	}
}

</script>

<template>
	<button :class="cls()" :disabled="disabled || busy" @click="$emit('click')">
		<img v-if="icon" :src="icon" class="icon" :style="iconStyle()" />
		<slot />
		<div v-if="busy" class="busy buttonBusy">
		</div>
	</button>
</template>

<style lang="scss" scoped>
.buttin {
	position: relative;
	display: flex;
	align-items: center;
}

.icon {
	width: 20px;
	height: 20px;
	margin-right: 8px;
}

.busy {
	position: absolute;
	width: 100%;
	height: 100%;
	left: 0;
	top: 0;
}
</style>
