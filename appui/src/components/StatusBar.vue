<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { fetchRegisteredServers, showMenu, switchToRegisteredServer } from '@/nattypes';
import type { Server } from '@/nattypes';

let servers = ref([] as Server[]);
let isMenuShown = ref(false);

function switchToServer(s: Server) {
	switchToRegisteredServer(s.publicKey);
}

function onMenu() {
	isMenuShown.value = !isMenuShown.value;
	showMenu(isMenuShown.value);
}

onMounted(async () => {
	servers.value = await fetchRegisteredServers();
})

</script>
 
<template>
	<div class="statusBar">
		<button @click="onMenu">Connect</button>
		<button v-for="s of servers" :key="s.publicKey" @click="switchToServer(s)">A</button>
	</div>
</template>

<style scoped lang="scss">
.statusBar {
	box-sizing: border-box;
	flex: 0 0 auto;
	// Our height must be perfectly in sync with the statusBarPlaceholder object in the Android app's
	// activity_main.xml
	height: 40px;
	//background-color: mistyrose;
	display: flex;
	align-items: center;
	padding: 2px 4px;
}
</style>
