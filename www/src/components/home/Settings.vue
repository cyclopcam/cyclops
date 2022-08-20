<script setup lang="ts">
</script>

<template>
	<div class="flexColumnCenter">
		<router-view v-slot="{ Component, route }">
			<transition :name="route.meta.transitionName">
				<component :is="Component" />
			</transition>
		</router-view>
	</div>
</template>

<style lang="scss" scoped>
</style>
