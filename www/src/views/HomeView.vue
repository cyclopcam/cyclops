<script setup lang="ts">
import ModeBar from '../components/home/ModeBar.vue';
</script>

<template>
	<div class="flexColumn home">
		<div class="flexColumn view">
			<router-view />
		</div>
		<mode-bar style="flex: 0 0 auto" />
	</div>
</template>

<style lang="scss" scoped>
.home {
	height: 100%;
	display: flex;
	flex-direction: column;
}

.view {
	align-items: center;
	height: 1px;
	flex: 1 1 auto;
	overflow: auto;
}
</style>
