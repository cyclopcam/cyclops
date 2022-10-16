<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { bestServerName, blankServer, getCurrentServer, showMenu, switchToRegisteredServer } from '@/nattypes';
import type { Server } from '@/nattypes';
import Menu from '@/icons/menu.svg';
import { globals } from '@/global';

//let currentServer = ref(blankServer());
//let servers = ref([] as Server[]);
let isMenuShown = ref(false);
let current = ref(blankServer());

function switchToServer(s: Server) {
	switchToRegisteredServer(s.publicKey);
}

function currentServerName(): string {
	return bestServerName(current.value);
}

function onMenu() {
	//isMenuShown.value = !isMenuShown.value;
	//globals.showMenu(isMenuShown.value);
	globals.showMenu(!globals.isFullScreen);
}

onMounted(async () => {
	//currentServer.value = await getCurrentServer();
	//servers.value = await fetchRegisteredServers();
	await globals.waitForLoad();
	current.value = globals.currentServer;
	//console.log("currentServerName = ", currentServerName());
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

	// Our StatusBar height must be perfectly in sync with the statusBarPlaceholder object in the Android app's
	// activity_main.xml
	height: 40px;

	background-color: #fff;
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
