<script setup lang="ts">
import { ref } from 'vue';


let props = defineProps<{
	icon: string, // svg drawn on the left of the text
	size?: string, // override default
}>()
defineEmits(['click']);

let root = ref(null);

function iconStyle(): any {
	let btn = root.value! as HTMLButtonElement;
	let danger = false;
	if (btn) {
		danger = btn.classList.contains("dangerButton");
	}

	return {
		width: props.size ?? "20px",
		height: props.size ?? "20px",
		filter: danger ? "invert(1) brightness(0.3) sepia(1) saturate(4) hue-rotate(-50deg)" : "",
		"margin-right": "8px",
	}
}

</script>

<template>
	<button ref="root" class="iconButton" @click="$emit('click')">
		<img :src="icon" :style="iconStyle()" />
		<slot />
	</button>
</template>

<style lang="scss" scoped>
.iconButton {
	display: flex;
	align-items: center;
}
</style>
