<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { RouterView } from 'vue-router';
import StatusBar from './components/StatusBar.vue';
import BitmapOverlay from './components/widgets/BitmapOverlay.vue';
import { globals } from './global';
import { getScreenParams } from './nattypes';

//let contentHeight = ref(0);
//
//function contentStyle() {
//	return {
//		// We have an absolute height, which is necessary for two things:
//		// 1. So that the moment our webview is expanded to fill the screen, our content is ready to be displayed (no white flash)
//		// 2. When the onscreen keyboard appears, and our webview's height is shrunk, we still display our background bitmap at the same size,
//		//    instead of shrinking it vertically, which looks really stupid.
//		"height": contentHeight.value + "px",
//		"z-index": 0,
//	}
//}
//
//onMounted(async () => {
//	let sp = await getScreenParams();
//	contentHeight.value = sp.contentHeight;
//})

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
</template>

<!-- These styles are not scoped, so importing 'base' affects all children, and they don't need to import it -->
<style lang="scss">
@import '@/assets/base.scss';

#app {
	height: 100%;
	display: flex;
	flex-direction: column;
	position: relative;
}

.container {
	width: 100%;
	height: 1px;
	flex: 1 1 auto;
	//background-color: antiquewhite;
	//display: flex;
	//align-items: center;
	//justify-content: center;
}
</style>	