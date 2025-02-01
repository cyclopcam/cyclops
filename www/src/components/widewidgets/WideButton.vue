<script setup lang="ts">
import { pushRoute } from "@/router/helpers";
import { useRouter } from "vue-router";
import right from '@/icons/chevron-right.svg';

let props = defineProps<{
	routeTarget?: string,
	rightArrow?: boolean, // By default we show a right arrow if routeTarget is defined, but you can override this with the 'rightArrow' prop
	disabled?: boolean,
	icon?: string,
	iconSize?: string,
	iconTweakX?: number, // Micro tweaks for different icon visual center-points
}>()

let emit = defineEmits(['click']);

const router = useRouter();

const defaultIconLeftMargin = 5;
const defaultIconRightMargin = 15;
const defaultIconSize = '30px';

function onClick() {
	if (!props.disabled) {
		emit('click');

		if (props.routeTarget) {
			if (props.routeTarget.includes('/')) {
				pushRoute(router, { path: props.routeTarget });
			} else {
				pushRoute(router, { name: props.routeTarget });
			}
		}
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
	<div :class="{ widewidget: true, widebutton: true, 'wide-section-element': true, centered: !icon && !routeTarget }"
		@click="onClick">
		<div v-if="icon" class="iconContainer">
			<img :src="icon" class="icon" :style="iconStyle()" />
		</div>
		<div v-if="routeTarget || rightArrow === true" class="inner inner-route">
			<slot />
			<img :src="right" />
		</div>
		<div v-else class="inner">
			<slot />
		</div>
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

.inner {
	display: flex;
	align-items: center;
	justify-content: center;
	flex-grow: 1;
	font-weight: 500;
	font-size: 15px;
}

.inner-route {
	justify-content: space-between;
}

.iconContainer {
	display: flex;
	align-items: center;
	justify-content: center;
	width: 60px;
	margin-right: 12px;
}

.icon {
	border-radius: 3px;
	object-fit: cover;
}
</style>