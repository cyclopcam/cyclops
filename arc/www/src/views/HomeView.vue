<script setup lang="ts">
import TopBar from '@/components/TopBar.vue';

</script>

<template>
	<div class="flexColumn home">
		<TopBar />
		<div class="homeBody">
			Foo! I am HomeView

		</div>
	</div>
</template>

<style scoped lang="scss">
.home {
	height: 100%;
}

.homeBody {
	flex: 1 1 auto;
}
</style>

