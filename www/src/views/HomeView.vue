<script setup lang="ts">
import Topbar from '../components/home/Topbar.vue';
</script>

<template>
	<div class="flexColumn home">
		<topbar style="flex: 0 0 auto" />
		<div class="flexColumn view">
			<router-view />
		</div>
	</div>
</template>

<style lang="scss" scoped>
.home {
	height: 100%;
}

.view {
	align-items: center;
	height: 1px;
	flex: 1 1 auto;
	overflow: auto;
}
</style>
