<script setup lang="ts">

let props = defineProps<{
	busy?: boolean, // show 'busy' animation. busy implies disabled.
	disabled?: boolean, // No need to set disabled if you already set busy.
	focal?: boolean, // One focal button per screen
}>()
defineEmits(['click']);

function cls() {
	return {
		buttin: true,
		focalButton: props.focal
	}
}

</script>

<template>
	<button :class="cls()" :disabled="disabled || busy" @click="$emit('click')">
		<slot />
		<div v-if="busy" class="busy buttonBusy">
		</div>
	</button>
</template>

<style lang="scss" scoped>
.buttin {
	position: relative;
}

.busy {
	position: absolute;
	width: 100%;
	height: 100%;
	left: 0;
	top: 0;
}
</style>
