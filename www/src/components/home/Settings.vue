<script setup lang="ts">
</script>

<template>
	<div class="flexColumn settingsRoot">
		<router-view v-slot="{ Component, route }">
			<transition :name="route.meta.transitionName + ''">
				<component :is="Component" />
			</transition>
		</router-view>
	</div>
</template>

<style lang="scss" scoped>
@import '@/assets/vars.scss';

.settingsRoot {
	box-sizing: border-box;
	height: 100%;

	// We do this, otherwise the invocation of a vertical scrollbar on desktop causes
	// there to be too little horizonta space, and then we get a really ugly horizontal
	// scrollbar too.
	overflow-x: hidden;

	overflow-y: auto;
	//padding: 10px 20px;

	// for desktop
	width: 300px;

	// for mobile
	@media (max-width: $mobileCutoff) {
		width: 100%;
	}
}
</style>
