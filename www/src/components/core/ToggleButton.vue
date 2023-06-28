<script setup lang="ts">
import { computed } from '@vue/reactivity';
import { pushRoute } from "@/router/helpers";
import { useRouter } from 'vue-router';

let props = defineProps<{
	icon: string,
	title: string,
	route: string, // Name of high-level route, such as rtTrain
	routeTarget?: string, // Name of deep route, such as rtTrainEditRecordings. If not specified, same as route
}>()

const router = useRouter();

let isSelected = computed(() => {
	// look through the hierarchy of matched routes, so that we include parent routes,
	// when we're deep down in the hierarchy.
	for (let m of router.currentRoute.value.matched) {
		if (m.name === props.route) {
			return true;
		}
	}
	return false;
});

function onClick() {
	pushRoute(router, { name: props.routeTarget ? props.routeTarget : props.route });
}
</script>

<template>
	<div :class="{ flexCenter: true, flexColumn: true, toggleButton: true, toggleSelected: isSelected }" @click="onClick">
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
	user-select: none;
}

.toggleButton:hover {
	border-color: #aad;
}

.title {
	margin-top: 4px;
	font-size: 10px;
}

.toggleSelected {
	background-color: $toggleColorMute;
}

.imgSelected {
	filter: invert(1);
}

.titleSelected {
	color: #fff;
}
</style>
