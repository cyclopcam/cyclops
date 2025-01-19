<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue';

let props = defineProps({
	// parent: relative to parent DOM element
	// previous: relative to previous sibling DOM element
	// center: center of screen
	position: {
		type: String,
		default: 'center'
	},

	// When position is either previous or center:
	// center: center of related DOM element
	// under: underneath related DOM element
	relative: {
		type: String,
		default: 'center',
	},

	// If  true, the make width the same as the width of the sizing 'position' element
	sameWidth: Boolean,

	// color that is applied to fullscreen screen-catcher
	// In addition to an rgba color, you can use these predefined colors:
	// extradark, dark, light
	tint: {
		type: String,
		default: 'rgba(0,0,0,0)'
	},

	// Poll the browser for our size, and shift around if we go off-screen
	pollSize: Boolean,

	// Show an 'x' in the top right
	showX: Boolean,

	// It's useful to turn this on when your modal control's size might exceed the screen's resolution
	scrollable: Boolean,

	// Make the parent object fill the screen, so that you can use width:100% and height:100% on your control
	fullscreen: Boolean,

	// Make this not a modal! This is useful when you want all the other features of this control, *except* for the modal nature
	// hmmm.. I can't get this to work. If we use pointer-events:none, then we never receive the mouse event that tells us to close ourselves.
	//clickThrough: Boolean,
})

let emits = defineEmits(['close']);

let domBreaker = ref(1);
let hide = ref(true);

let screenHeight = ref(0);
let ownWidth = ref(0);
let ownHeight = ref(0);
let numPolls = 0;
let xLeft = ref(0);
let xTop = ref(0);

const fixed = ref(null);
const container = ref(null);
const xButton = ref(null);

function fixedStyle(): any {
	let t = props.tint;
	if (t === 'extradark')
		t = 'rgba(0,0,0,0.5)';
	else if (t === 'dark')
		t = 'rgba(0,0,0,0.3)';
	else if (t === 'mild')
		t = 'rgba(0,0,0,0.1)';
	else if (t === 'light')
		t = 'rgba(255,255,255,0.3)';

	let s: any = {
		'background-color': t,
	};
	if (screenHeight.value > 0) {
		s['height'] = screenHeight.value + 'px';
	}
	//if (props.clickThrough) {
	//	s['pointer-events'] = "none";
	//}
	return s;
}

function containerStyle(): any {
	let s: any = {};
	if (domBreaker.value > 9e9) {
		// provide an escape hatch to force recomputation of containerStyle once we get mounted
		s['left'] = 0;
	}
	if (hide.value) {
		s['visibility'] = 'hidden';
	}
	if (props.scrollable) {
		s['overflow'] = 'auto';
		s['max-width'] = '100%';
		s['max-height'] = '100%';
	}
	if (props.fullscreen) {
		s['width'] = '100%';
		s['height'] = '100%';
		if (props.position === 'center') {
			s['display'] = "flex";
			s['align-items'] = "center";
			s['justify-content'] = "center";
		}
	}

	let fixedEl = fixed.value! as HTMLDivElement;

	if (fixedEl && (props.position === 'previous' || props.position === 'parent')) {
		//console.log("got it");

		let center = props.relative === 'center';
		let under = !center;
		let screenW = document.documentElement.clientWidth;
		let screenH = document.documentElement.clientHeight;
		let refr: DOMRect; // reference rectangle
		if (props.position === 'previous') {
			refr = fixedEl.previousElementSibling!.getBoundingClientRect();
			//console.log("Previous", refr);
		} else {
			refr = fixedEl.parentElement!.getBoundingClientRect();
		}
		let myWidth = ownWidth.value;
		let myHeight = ownHeight.value;
		//console.log(`myWidth: ${myWidth}, myHeight: ${myHeight}`);
		if (props.sameWidth) {
			myWidth = refr.width;
		}
		let pad = 10;
		let left = refr.x + refr.width / 2 - myWidth / 2;
		let top = refr.y + refr.height / 2 - myHeight / 2;
		if (under) {
			top = refr.bottom;
		}
		if (left < pad)
			left = pad;
		if (top < pad)
			top = pad;
		if (left + myWidth + pad > screenW)
			left = screenW - myWidth - pad;
		if (top + myHeight + pad > screenH)
			top = screenH - myHeight - pad;
		s['position'] = 'absolute';
		s['left'] = left + 'px';
		s['top'] = top + 'px';
		if (props.sameWidth) {
			s['min-width'] = myWidth + 'px';
		}
	}
	return s;
}

function xStyle(): any {
	return {
		"left": xLeft.value + "px",
		"top": xTop.value + "px",
	};
}

function refreshOwnSize() {
	let self = container.value! as HTMLElement;
	if (!self)
		return;
	let rect = self.getBoundingClientRect();
	//if (rect.width !== this.ownWidth || rect.height !== this.ownHeight) {
	//	console.log(`Modal detected altered size. numPolls = ${this.numPolls}`);
	//}
	ownWidth.value = rect.width;
	ownHeight.value = rect.height;
}

function refreshXPosition() {
	let x = xButton.value! as HTMLElement;
	if (!x)
		return;
	let containerEl = container.value! as HTMLElement;
	if (!containerEl)
		return;
	let slot = containerEl.firstElementChild;
	if (!slot || slot === x)
		return;
	let slotRect = slot.getBoundingClientRect();
	xLeft.value = slotRect.right - 33;
	xTop.value = slotRect.top + 7;
}

function xPoller() {
	refreshXPosition();
	setTimeout(xPoller, 200);
}

function onRootClick(ev: MouseEvent) {
	if (ev.target === fixed.value) {
		// outside click
		if (!props.showX) {
			emits('close');
		}
	}
}

function onXClick() {
	emits('close');
}

function sizePoller() {
	//console.log(`sizePoller ${numPolls}`);
	numPolls++;
	refreshOwnSize();
	// Give the DOM one or two cycles to stabilize the layout. By not drawing ourselves for 5 cycles,
	// we end up avoiding a visible flicker, should the menu need to move itself.
	let showAfterN = 5;
	if (numPolls === showAfterN) {
		hide.value = false;
	}
	let timeout = numPolls <= showAfterN ? 5 : 100;
	setTimeout(() => { sizePoller() }, timeout);
}

// This is for adapting to screen size change when virtual keyboard is shown
function onScreenSizeChanged() {
	saveScreenHeight();
}

function saveScreenHeight() {
	screenHeight.value = window.visualViewport ? window.visualViewport.height : window.innerHeight;
}

onMounted(() => {
	domBreaker.value++;
	saveScreenHeight();
	window.addEventListener('resize', onScreenSizeChanged);
	if (props.pollSize) {
		sizePoller();
	} else {
		refreshOwnSize();
		hide.value = false;
	}
	if (props.showX)
		xPoller();
})

onUnmounted(() => {
	window.removeEventListener('resize', onScreenSizeChanged);
})

</script>

<template>
	<div ref="fixed" :class="{ modalRoot: true, centered: true }" :style="fixedStyle()" @mousedown="onRootClick">
		<div ref="container" :style="containerStyle()">
			<slot />
			<div v-if="showX" ref="xButton" class="x" :style="xStyle()" @click="onXClick"></div>
		</div>
	</div>
</template>

<style lang="scss" scoped>
.modalRoot {
	position: fixed;
	left: 0;
	top: 0;
	width: 100%;
	//height: 100%; // We override this in code, in order to deal with virtual keyboards on mobile
	transition: height 60ms;
	z-index: 1;
}

.centered {
	display: flex;
	justify-content: center;
	align-items: center;
}

.x {
	position: absolute;
	width: 26px;
	height: 26px;
	background-position: center;
	background-repeat: no-repeat;
	background-size: 22px 22px;
	background-image: url('@/icons/x.svg');
	cursor: pointer;
}

.x:hover {
	background-size: 25px 25px;
}
</style>
