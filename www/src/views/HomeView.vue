<script setup lang="ts">
import ModeBar from '../components/home/ModeBar.vue';
</script>

<template>
	<div class="flexColumn home">
		<div class="flexColumn view">
			<router-view />
		</div>
		<mode-bar class="modeBarInHome" />
	</div>
</template>

<style lang="scss" scoped>
@import '@/assets/vars.scss';

.home {
	height: 100%;
	display: flex;
	flex-direction: column;
	// start new stacking context, so that mode-bar can have a positive z-index.
	// We want the mode-bar to be above other content, so that when we animate pages
	// during transitions, we don't draw over the mode bar.
	z-index: 0;
}

.view {
	align-items: center;
	height: 1px;
	flex: 1 1 auto;
	//overflow: auto;
	z-index: 0; // lower than mode-bar
	//background-color: $darkBackground1;
}

.modeBarInHome {
	flex: 0 0 auto;
	z-index: 1;
}
</style>
