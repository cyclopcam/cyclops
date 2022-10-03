<script setup lang="ts">
import { globals } from '@/natcom';
import Init from './components/Init.vue';
import Login from './components/Login.vue';

type StringMap = { [key: string]: string };

function getMode(): string {
	return globals.mode;
}
</script>

<template>
	<login v-if="getMode() === 'login'" />
	<init v-else-if="getMode() === 'init'" />
	<div v-else>Unknown mode!</div>
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