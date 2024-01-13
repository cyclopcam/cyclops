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
	width: 300px;
	//padding: 10px 20px;

	@media (max-width: $mobileCutoff) {
		width: 100%;
	}
}
</style>
