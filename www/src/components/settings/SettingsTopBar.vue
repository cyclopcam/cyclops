<script setup lang="ts">
// It turns out we don't need this, because we ALREADY have a top bar on mobile,
// which is actually a slim native webui that talks directly to the mobile device.
let props = defineProps<{
	title: string
}>()
</script>

<template>
	<div class="settingsTopBar">
		{{ title }}
	</div>
</template>

<style lang="scss" scoped>
.settingsTopBar {
	box-sizing: border-box;
	width: 100%;
	height: 40px;
	background-color: #333;
	color: white;
	display: flex;
	align-items: center;
	padding: 0 10px;
	font-size: 14px;
}
</style>
