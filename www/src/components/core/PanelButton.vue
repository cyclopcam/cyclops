<script setup lang="ts">
import { pushRoute } from "@/router/helpers";
import right from '@/icons/chevron-right.svg';
import { useRouter } from "vue-router";

let props = defineProps<{
	routeTarget: string,
	mode?: string, // list (default), solo. (list = column of buttons, solo = a single button on it's own)
	icon?: string,
	iconSize?: string,
	iconTweakX?: number, // Micro tweaks for different icon visual center-points
}>()

const router = useRouter();

const defaultIconLeftMargin = 5;
const defaultIconRightMargin = 15;
const defaultIconSize = '30px';

function isList(): boolean {
	return props.mode !== 'solo';
}

function isSolo(): boolean {
	return props.mode === 'solo';
}

function onClick() {
	if (props.routeTarget.includes('/')) {
		pushRoute(router, { path: props.routeTarget });
	} else {
		pushRoute(router, { name: props.routeTarget });
	}
}

function iconStyle(): any {
	return {
		width: props.iconSize ?? defaultIconSize,
		height: props.iconSize ?? defaultIconSize,
		"margin-left": (props.iconTweakX ?? 0) + defaultIconLeftMargin + 'px',
		"margin-right": defaultIconRightMargin - (props.iconTweakX ?? 0) + 'px',
	}
}

</script>

<template>
	<div :class="{ panelButton: true, list: isList(), solo: isSolo(), shadow5LHover: isSolo() }" @click="onClick">
		<div class="iconContainer">
			<img v-if='icon' :src="icon" class="icon" :style="iconStyle()" />
		</div>
		<div class="inner">
			<slot />
			<img :src="right" />
		</div>
	</div>
</template>

<style lang="scss" scoped>
@import '@/assets/vars.scss';

.panelButton {
	padding: 5px 10px 5px 5px;
	height: 60px;

	@media (max-width: $mobileCutoff) {
		padding-left: 15px;
	}

	cursor: pointer;
	display: flex;
	align-items: center;
}

.list {
	border-bottom: solid 1px #ddd;
}

.solo {
	border: solid 1px #ddd;
}

.inner {
	display: flex;
	align-items: center;
	justify-content: space-between;
	flex-grow: 1;
	font-weight: 500;
}

.iconContainer {
	display: flex;
	align-items: center;
	justify-content: center;
	width: 70px;
	margin-right: 10px;
}

.icon {
	border-radius: 3px;
	object-fit: cover;
}
</style>
