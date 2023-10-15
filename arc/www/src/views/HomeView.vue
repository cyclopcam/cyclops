<script setup lang="ts">
import TopBar from '@/components/TopBar.vue';

</script>

<template>
	<div class="flexColumn home">
		<TopBar />
		<RouterView class="homeContent" />
	</div>
</template>

<style scoped lang="scss">
.home {
	height: 100%;
}

.homeContent {
	flex: 1 1 auto;
}
</style>

