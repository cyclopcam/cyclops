<script setup lang="ts">
// This is used to allow a mobile app to cover 100% of the screen, while giving the
// appearance that there's native mobile content underneath. We take a screengrab
// of the native content, and then display that as an underlay, and we render
// all of our UI on top.
// All of this is necessary because an Android WebView can't display partially
// transparent content. It is a rectangle that is drawn with 100% opacity.

import { panelSlideTransitionMS } from '@/constants';
import { globals } from '@/global';
import { onMounted, ref, watch } from 'vue';

let canvas = ref(null);
let contentParent = ref(null);
let contentParentTop = ref("");
let enableTopTransition = ref(false);
let isHidingFromSwipe = ref(false);
let darkenOpacity = ref("");
let isTouchDown = false;
let touchDownAt = 0;
let lastTouchPos = 0;
let lastTouchTimeMS = 0;
let lastTouchSpeed = 0;
let swipeTransitionMS = 150;
//let touchDelta = ref(0);

function canvasStyle() {
	return {
		// We have an absolute height, which is necessary for two things:
		// 1. So that the moment our webview is expanded to fill the screen, our content is ready to be displayed.
		//    If we didn't have this size up front, we'd need to wait for the webview to made large, then it will issue
		//    a layout.. and then finally we'll fill it. But that causes a white flash.
		// 2. When the onscreen keyboard appears, and our webview's height is shrunk, we still display our background bitmap at the same size,
		//    instead of shrinking it vertically, which looks really stupid.
		"height": globals.contentHeight + "px",
	}
}

function contentParentStyle() {
	let transition = "top 0ms";
	if (enableTopTransition.value) {
		transition = "top " + (isHidingFromSwipe.value ? swipeTransitionMS + "ms" : panelSlideTransitionMS + "ms");
	}
	return {
		transition: transition,
		top: contentParentTop.value !== "" ? contentParentTop.value : undefined,
	}
}

function darkenStyle() {
	return {
		transition: "opacity " + swipeTransitionMS + "ms",
		opacity: darkenOpacity.value !== "" ? darkenOpacity.value : undefined,
	}
}

watch(() => globals.hideFullScreen, () => {
	//console.log("globals.hideFullScreen changed to ", globals.hideFullScreen);
	if (globals.hideFullScreen && !isHidingFromSwipe.value) {
		hideContent(false);
	}
});

function hideContent(fromSwipe: boolean) {
	enableTopTransition.value = true;
	contentParentTop.value = -(contentParent.value! as HTMLDivElement).clientHeight + "px";
	isHidingFromSwipe.value = fromSwipe;
	darkenOpacity.value = "0";
	if (fromSwipe) {
		setTimeout(() => {
			globals.showMenu(false, { immediateHide: true });
		}, swipeTransitionMS);
	}
}

function showContent() {
	enableTopTransition.value = true;
	contentParentTop.value = "0px";
	darkenOpacity.value = "0.3";
}

// returns the speed in pixels per second
function recordLastTouchPos(ev: TouchEvent): number {
	let dp = ev.touches[0].clientY - lastTouchPos;
	let dt = ev.timeStamp - lastTouchTimeMS;
	lastTouchPos = ev.touches[0].clientY;
	lastTouchTimeMS = ev.timeStamp;
	return dp / (dt / 1000);
}

function onTouchStart(ev: TouchEvent) {
	//ev.preventDefault();
	isTouchDown = true;
	touchDownAt = ev.touches[0].clientY;
	recordLastTouchPos(ev);
}

function onTouchMove(ev: TouchEvent) {
	//ev.preventDefault();
	if (isTouchDown) {
		let delta = Math.min(0, ev.touches[0].clientY - touchDownAt);
		enableTopTransition.value = false;
		contentParentTop.value = delta + "px";
		lastTouchSpeed = recordLastTouchPos(ev);
		//touchDelta.value = ev.touches[0].clientY - touchDownAt;
	}
}

function onTouchEnd(ev: TouchEvent) {
	//ev.preventDefault();
	isTouchDown = false;
	enableTopTransition.value = true;
	contentParentTop.value = "0px";
	let msSinceTouchMove = ev.timeStamp - lastTouchTimeMS;
	//console.log("speed at touchEnd", lastTouchSpeed, "ms since touchmove", msSinceTouchMove);
	if (lastTouchSpeed < -100 && msSinceTouchMove < 200) {
		hideContent(true);
	}
}

onMounted(() => {
	if (globals.fullScreenBackdrop) {
		let cc = canvas.value! as HTMLCanvasElement;
		cc.width = globals.fullScreenBackdrop.width;
		cc.height = globals.fullScreenBackdrop.height;
		let cx = cc.getContext('2d')!;
		cx.putImageData(globals.fullScreenBackdrop, 0, 0);
		console.log("BitmapOverlay canvas dims", cc.clientWidth, cc.clientHeight, ", Bitmap dims", globals.fullScreenBackdrop.width, globals.fullScreenBackdrop.height);
	}
	// Measure the height of our content pane, so that we can slide it precisely out of 
	// view, and then precisely into view.
	//let contentDiv = contentParent.value! as HTMLDivElement;
	//console.log("slotEl", contentDiv.firstElementChild!.clientHeight);
	//console.log("slotEl", contentDiv!.clientHeight);

	// Start out of view
	//contentParentTop.value = -contentDiv.firstElementChild!.clientHeight + "px";
	//contentParentTop.value = -contentDiv.clientHeight + "px";
	hideContent(false);

	// Slide into view
	setTimeout(() => {
		//contentParentTop.value = "0px";
		showContent();
	})
});

</script>

<template>
	<div class="bitmapOverlay">
		<canvas ref="canvas" class="canvas" :style="canvasStyle()" />
		<div ref="darken" class="darken" :style="darkenStyle()" />
		<div ref="contentParent" class="contentParent" :style="contentParentStyle()" @touchstart="onTouchStart"
			@touchmove="onTouchMove" @touchend="onTouchEnd">
			<slot />
		</div>
	</div>
</template>

<style lang="scss" scoped>
.bitmapOverlay {
	width: 100%;
	height: 100%;
	position: relative;
}

.canvas {
	position: absolute;
	left: 0;
	top: 0;
	width: 100%;
	height: 100%;
	overflow: visible;
	//filter: brightness(0.7);
}

.darken {
	position: absolute;
	left: 0;
	top: 0;
	width: 100%;
	height: 100%;
	//background-color: #000;
}

.contentParent {
	position: absolute;
	left: 0;
	width: 100%;
	box-shadow: 0 0 25px rgba(0, 0, 0, 0.7);
}
</style>
