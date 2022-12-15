<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { bestServerName, blankServer, natGetLastServer, natSetLocalWebviewVisibility, natSwitchToRegisteredServer } from '@/nativeOut';
import type { Server } from '@/nativeOut';
import Menu from '@/icons/menu.svg';
import { globals } from '@/global';

function switchToServer(s: Server) {
	natSwitchToRegisteredServer(s.publicKey);
}

function isFullScreen(): boolean {
	return globals.isFullScreen;
}

function currentServerName(): string {
	return bestServerName(globals.currentServer);
}

function onMenu() {
	if (globals.mustShowWelcomeScreen && globals.isFullScreen) {
		// just do nothing, because hiding ourselves would just show a blank white page
		return;
	}
	globals.showExpanded(!globals.isFullScreen);
}

</script>
 
<template>
	<div class="statusBar">
		<img :src="Menu" @click="onMenu" class="menu" draggable="false" />
		<div class="middle">
			{{ currentServerName() }}
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
	// SYNC-STATUS-BAR-HEIGHT
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
