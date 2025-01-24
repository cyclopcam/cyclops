<script setup lang="ts">
</script>

<template>
	<div id="settingsRoot" class="flexColumn settingsRoot">
		<router-view v-slot="{ Component, route }">
			<transition :name="route.meta.transitionName + ''">
				<component :is="Component" />
			</transition>
		</router-view>
	</div>
</template>

<style lang="scss" scoped>
//@import '@/assets/vars.scss';
@import '@/assets/base.scss';

.settingsRoot {
	box-sizing: border-box;
	background: $widerootbg;
	height: 100%;

	// We do this, otherwise the invocation of a vertical scrollbar on desktop causes
	// there to be too little horizontal space, and then we get a really ugly horizontal
	// scrollbar too.
	overflow-x: hidden;

	overflow-y: auto;

	// for desktop
	width: 300px;

	// for mobile
	@media (max-width: $mobileCutoff) {
		width: 100%;
	}
}
</style>
