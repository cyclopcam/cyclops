<script setup lang="ts">
import { pushRoute, router } from '@/router/routes';
import right from '@/icons/chevron-right.svg';

let props = defineProps<{
	routeTarget: string,
	mode?: string, // list (default), solo. (list = column of buttons, solo = a single button on it's own)
	icon?: string,
	iconSize?: string,
}>()

function isList(): boolean {
	return props.mode !== 'solo';
}

function isSolo(): boolean {
	return props.mode === 'solo';
}

function onClick() {
	pushRoute({ name: props.routeTarget });
}

function iconStyle(): any {
	return {
		width: props.iconSize ?? '24px',
		height: props.iconSize ?? '24px',
	}
}

</script>

<template>
	<div :class="{ panelButton: true, list: isList(), solo: isSolo(), shadow5LHover: isSolo() }" @click="onClick">
		<img v-if='icon' :src="icon" class="icon" :style="iconStyle()" />
		<div v-else style='width:10px' />
		<div class="inner">
			<slot />
			<img :src="right" />
		</div>
	</div>
</template>

<style lang="scss" scoped>
@import '@/assets/vars.scss';

.panelButton {
	padding: 15px 10px 15px 5px;

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
}

.icon {
	margin-left: 5px;
	margin-right: 15px;
}
</style>
