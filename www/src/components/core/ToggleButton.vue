<script setup lang="ts">
import router from '@/router/routes.js';
import { computed } from '@vue/reactivity';

let props = defineProps<{
	icon: string,
	title: string,
	route: string,
}>()

let isSelected = computed(() => {
	return router.currentRoute.value.name === props.route;
});

function onClick() {
	router.push({ name: props.route });
}
</script>

<template>
	<div :class="{ flexCenter: true, flexColumn: true, toggleButton: true, toggleSelected: isSelected }"
		@click="onClick">
		<img :src="icon" :class="{ imgSelected: isSelected }" />
		<div :class="{ title: true, titleSelected: isSelected }">{{ title }}</div>
	</div>
</template>

<style lang="scss" scoped>
@import '@/assets/vars.scss';

.toggleButton {
	cursor: pointer;
	border: solid 1px rgba(0, 0, 0, 0);
	padding: 4px 6px;
	border-radius: 5px;
}

.toggleButton:hover {
	border-color: #aad;
	;
}

.title {
	margin-top: 4px;
	font-size: 10px;
}

.toggleSelected {
	background-color: $toggleColor;
}

.imgSelected {
	filter: invert(1);
}

.titleSelected {
	color: #fff;
}
</style>
