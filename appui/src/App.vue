<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { RouterView } from 'vue-router';
import StatusBar from './components/StatusBar.vue';
import BitmapOverlay from './components/widgets/BitmapOverlay.vue';
import { globals, ServerPort } from './global';
import { blankServer, natGetScreenParams } from './nativeOutt';

let current = ref(blankServer());

function currentURL() {
	if (current.value.publicKey === "") {
		return "";
	}
	return "http://" + current.value.lanIP + ":" + ServerPort;
}

//function isFullScreen(): boolean {
//	console.log("App.vue isFullScreen: ", globals.isFullScreen);
//	return globals.isFullScreen;
//}

onMounted(async () => {
	console.log("App.vue onMounted start");
	await globals.waitForLoad();
	current.value = globals.currentServer;
	console.log("App.vue onMounted end");
})

</script>

<template>
	<status-bar style="z-index: 1" />
	<div v-if="globals.isFullScreen" class="container">
		<bitmap-overlay>
			<router-view v-slot="{ Component, route }">
				<!-- The "+ ''" is merely here to satisfy the compiler. I don't know why this doesn't work out of the box. -->
				<transition :name="route.meta.transitionName + ''">
					<component :is="Component" />
				</transition>
			</router-view>
		</bitmap-overlay>
	</div>
	<!--<iframe v-else class="remote" :src="currentURL()"></iframe> -->
</template>

<!-- These styles are not scoped, so importing 'base' affects all children, and they don't need to import it -->
<style lang="scss">
@import '@/assets/base.scss';
@import '@/assets/base-appui.scss';

#app {
	height: 100%;
	display: flex;
	flex-direction: column;
	position: relative;
}

.container {
	width: 100%;
	// Rather let BitmapOverlay inform us of it's size
	//height: 1px;
	//flex: 1 1 auto;
}

//.remote {
//	width: 100%;
//	height: 300px;
//}
</style>	