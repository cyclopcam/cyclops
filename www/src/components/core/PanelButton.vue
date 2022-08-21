<script setup lang="ts">
import router from '@/router/routes';
import right from '@/icons/chevron-right.svg';

let props = defineProps<{
	routeTarget: string,
	icon?: string,
	iconSize?: string,
}>()

function onClick() {
	router.push({ name: props.routeTarget });
}

function iconStyle(): any {
	return {
		width: props.iconSize ?? '24px',
		height: props.iconSize ?? '24px',
	}
}

</script>

<template>
	<div class="panelButton" @click="onClick">
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
	//background-color: aquamarine;
	border-bottom: solid 1px #ddd;
	padding: 15px 10px 15px 5px;

	@media (max-width: $mobileCutoff) {
		padding-left: 15px;
	}

	cursor: pointer;
	display: flex;
	align-items: center;
}

//.panelButton:first-child {
//	border-top: solid 1px #ddd;
//}

.inner {
	display: flex;
	align-items: center;
	justify-content: space-between;
	flex-grow: 1;
}

.icon {
	margin-right: 15px;
}
</style>
