<script setup lang="ts">
import { globals } from '@/natcom';
import Init from './components/Init.vue';
import Login from './components/Login.vue';

type StringMap = { [key: string]: string };

/*
function urlKV(): StringMap {
	let search = window.location.search;
	if (search.length <= 1) {
		return {};
	}
	// window.location.search starts with ?, so chop it off, then split
	let pairs = window.location.search.substring(1).split("&");
	let m: StringMap = {};
	for (let p of pairs) {
		let lr = p.split("=");
		if (lr.length === 2) {
			m[lr[0]] = lr[1];
			console.log(lr[0], lr[1]);
		}
	}
	return m;
}
*/

function getMode(): string {
	return globals.mode;
}
</script>

<template>
	<login v-if="getMode() === 'login'" />
	<init v-else-if="getMode() === 'init'" />
	<div v-else>Unknown mode!</div>
</template>

<!-- NOTE! These styles are not scoped, which is why we import 'base' -->
<style lang="scss">
#app {
	height: 100%;
	display: flex;
	align-items: center;
	justify-content: center;
}

@import '@/assets/base.scss';
</style>	