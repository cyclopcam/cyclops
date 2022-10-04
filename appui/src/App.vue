<script setup lang="ts">
import { RouterView } from 'vue-router';
</script>

<template>
	<router-view v-slot="{ Component, route }">
		<!-- The "+ ''" is merely here to satisfy the compiler. I don't know why this doesn't work out of the box. -->
		<transition :name="route.meta.transitionName + ''">
			<component :is="Component" />
		</transition>
	</router-view>

</template>

<!-- These styles are not scoped, so importing 'base' affects all children, and they don't need to import it -->
<style lang="scss">
@import '@/assets/base.scss';

#app {
	height: 100%;
	display: flex;
	align-items: center;
	justify-content: center;
}
</style>	