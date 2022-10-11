<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { bestServerName, blankServer, fetchRegisteredServers, getCurrentServer, showMenu, switchToRegisteredServer } from '@/nattypes';
import type { Server } from '@/nattypes';
import Menu from '@/icons/menu.svg';

let currentServer = ref(blankServer());
let servers = ref([] as Server[]);
let isMenuShown = ref(false);

function switchToServer(s: Server) {
	switchToRegisteredServer(s.publicKey);
}

function currentServerName(): string {
	return bestServerName(currentServer.value);
}

function onMenu() {
	isMenuShown.value = !isMenuShown.value;
	showMenu(isMenuShown.value);
}

onMounted(async () => {
	currentServer.value = await getCurrentServer();
	servers.value = await fetchRegisteredServers();
})

</script>
 
<template>
	<div class="statusBar">
		<img :src="Menu" @click="onMenu" class="menu" />
		<!-- <button v-for="s of servers" :key="s.publicKey" @click="switchToServer(s)">A</button> -->
		<div class="middle">
			{{currentServerName()}}
		</div>
		<div class="right">
		</div>
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
	justify-content: space-between;
	padding: 2px 4px;

	border-bottom: solid 1px #ccc;
}

.menu {
	margin: 2px 2px 2px 4px;
	width: 26px;
	height: 26px;
}

.middle {
	font-size: 15px;
}

.right {
	width: 30px; // to balance burger on the left, and make central text centered
}
</style>
