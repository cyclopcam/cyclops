<script setup lang="ts">
import Topbar from '../components/home/Topbar.vue';
</script>

<template>
	<div>
		<topbar />
		<div class="flexColumn view">
			<router-view />
		</div>
	</div>
</template>

<style lang="scss" scoped>
.view {
	align-items: center;
}
</style>
