<script setup lang="ts">
</script>

<template>
	<div id="settingsRoot" class="flexColumn wideRootInner">
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
</style>
