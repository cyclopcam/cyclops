<script setup lang="ts">
let props = defineProps<{
	size: string,
	hotZone: boolean,
}>()

function paintBlocksStyle() {
	return {
		width: props.size,
		height: props.size
	}
}
</script>

<template>
	<div class="paintBlocks" :style="paintBlocksStyle()">
		<div style="left: 0%; top: 0%" :class="{ 'hot': hotZone, 'ignore': !hotZone }"></div>
		<div style="left: 50%; top: 0%" :class="{ 'hot': hotZone, 'ignore': !hotZone }"></div>
		<div style="left: 0%; top: 50%" :class="{ 'hot': hotZone, 'ignore': !hotZone }"></div>
		<div style="left: 50%; top: 50%" :class="{ 'hot': hotZone, 'ignore': !hotZone }"></div>
	</div>
</template>

<style lang="scss" scoped>
.paintBlocks {
	position: relative;
	width: 40px;
	height: 40px;
}

.paintBlocks>div {
	box-sizing: border-box;
	position: absolute;
	width: 50%;
	height: 50%;
}

.hot {
	border: solid 1px #a22;
	background-color: #f88;
}

.ignore {
	border: solid 1px rgb(168, 168, 168);
	background-color: rgb(236, 236, 236);
}
</style>
