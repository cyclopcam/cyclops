<script setup lang="ts">
let props = defineProps<{
	padding?: string, // default: 2px 16px
}>()
</script>

<template>
	<div class="widesection" :style="{ padding: props.padding || '2px 16px' }">
		<slot />
	</div>
</template>

<style lang="scss" scoped>
@import '@/assets/vars.scss';
@import './widewidget.scss';

.widesection {
	width: 100%;
	box-sizing: border-box;
	padding: 2px 16px;
	margin-bottom: 8px;
	background-color: #fff;
	box-shadow: 0px 1px 3px rgba(0, 0, 0, 0.08);
}

/*
 We rather opt in with an explicit class .wide-section-element
 Otherwise we find ourselves having to opt out too often, and we get
 clobbered by CSS precedence rules, which leads to !important, and
 once you end up there you've gone wrong.
.widesection> :first-child {
	border-top: solid 1px rgba(0, 0, 0, 0);
}
.widesection> :not(:first-child) {
	border-top: solid 1px #f4f4f4;
}
*/
</style>