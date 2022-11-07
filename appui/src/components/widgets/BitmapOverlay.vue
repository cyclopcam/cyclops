<script setup lang="ts">
// This is used to allow a mobile app to cover 100% of the screen, while giving the
// appearance that there's native mobile content underneath. We take a screengrab
// of the native content, and then display that as an underlay, and we render
// all of our UI on top.
// All of this is necessary because an Android WebView can't display partially
// transparent content. It is a rectangle that is drawn with 100% opacity.

import { panelSlideTransitionMS } from '@/constants';
import { globals } from '@/global';
import { LocalWebviewVisibility, natSetLocalWebviewVisibility } from '@/nativeOut';
import { onMounted, ref, watch } from 'vue';

let canvas = ref(null);
let contentParent = ref(null);
let contentParentTop = ref("");
let contentParentOpacity = ref("1");
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

// On globals.contentHeight:
// We have an absolute height, which is necessary for two things:
// 1. So that the moment our webview is expanded to fill the screen, our content is ready to be displayed.
//    If we didn't have this size up front, we'd need to wait for the webview to be made large, then it will issue
//    a layout.. and then finally we'll fill it. But that causes a white flash.
// 2. When the onscreen keyboard appears, and our webview's height is shrunk, we still display our background bitmap at the same size,
//    instead of shrinking it vertically, which looks really stupid. By declaring a fixed size, this shrinkage doesn't happen.

function rootStyle() {
	return {
		"height": globals.contentHeight + "px",
	}
}

// Make the container take up 100% of the screen (i.e nothing underneath)
// In your component (eg AddLocal.vue), you must ALSO set height:100%, to make this work.
function isFullScreenContainer(): boolean {
	return globals.mustShowWelcomeScreen;
}

function allowSwipeAway(): boolean {
	return !isFullScreenContainer();
}

function contentParentStyle() {
	let transition = "top 0ms";
	let tms = (isHidingFromSwipe.value ? swipeTransitionMS + "ms" : panelSlideTransitionMS + "ms");
	if (enableTopTransition.value) {
		transition = "top " + tms + ", opacity " + tms;
	}
	return {
		transition: transition,
		top: contentParentTop.value !== "" ? contentParentTop.value : undefined,
		opacity: contentParentOpacity.value !== "" ? contentParentOpacity.value : undefined,
		height: isFullScreenContainer() ? "100%" : undefined,
	}
}

function darkenStyle() {
	return {
		transition: enableTopTransition.value ? "opacity " + swipeTransitionMS + "ms" : "",
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
	contentParentOpacity.value = "0";
	if (fromSwipe) {
		setTimeout(() => {
			globals.showExpanded(false, { immediateHide: true });
		}, swipeTransitionMS);
	}
}

function showContent() {
	enableTopTransition.value = true;
	contentParentOpacity.value = "100%";
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
	if (allowSwipeAway()) {
		isTouchDown = true;
		touchDownAt = ev.touches[0].clientY;
		recordLastTouchPos(ev);
	}
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

	console.log("BitmapOverlay mounted");

	// hide, while it's in view, otherwise we get a white flash, until contentRenderPause happens
	contentParentOpacity.value = "0";
	darkenOpacity.value = "0";

	// Start out of view
	// Pause first, so that our content can get it's height
	let contentRenderPause = 5;
	setTimeout(() => {
		hideContent(false);
	}, contentRenderPause);

	// settlePause is waiting for our content to be rendered in our expanded WebView, to try and
	// avoid the white flash. But so far I can't get it consistent.
	let settlePause = 50; // + 5 * 1000;

	setTimeout(() => {
		// Ask Android to make our WebView visible again
		console.log("BitmapOverlay showMenu 2");
		//console.log("contentHeight = ", (contentParent.value! as HTMLDivElement).clientHeight);
		natSetLocalWebviewVisibility(LocalWebviewVisibility.Show);
	}, contentRenderPause + settlePause);

	// Slide our content into view, from the top
	setTimeout(() => {
		console.log("BitmapOverlay slide down");
		//contentParentTop.value = "0px";
		showContent();
	}, contentRenderPause + settlePause + 10);
});

</script>

<template>
	<div class="bitmapOverlay" :style="rootStyle()">
		<canvas ref="canvas" class="canvas" />
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
	//height: 100%; // height is defined by our rootStyle() function
	position: relative;
}

.canvas {
	position: absolute;
	left: 0;
	top: 0;
	width: 100%;
	height: 100%;
	overflow: visible;
	//filter: hue-rotate(90deg);
	//filter: brightness(0.7);
}

.darken {
	position: absolute;
	left: 0;
	top: 0;
	width: 100%;
	height: 100%;
	background-color: #000;
}

.contentParent {
	position: absolute;
	left: 0;
	width: 100%;
	box-shadow: 0 0 25px rgba(0, 0, 0, 0.7);
}
</style>
