<script setup lang="ts">
import type { CameraInfo } from '@/camera/camera';
import Player from '@/components/camera/Player.vue';
import { ref } from 'vue';

let props = defineProps<{
	camera: CameraInfo,
	play: boolean,
	width?: string,
	height?: string,
	icon?: string, // 'play', 'record' (default = play)
}>()
defineEmits(['play', 'stop']);

// This is only useful if the camera is not showing anything (i.e. we can't connect to it),
// but how to detect that? I guess we need an API for that.
let showCameraName = ref(false);

function style(): any {
	return {
		"width": props.width,
		"height": props.height,
	}
}
function iconIsPlay() { return (props.icon ?? "play") === "play"; }
function iconIsRecord() { return (props.icon ?? "play") === "record"; }

</script>

<template>
	<div class="flex cameraItem" :style="style()">
		<player :camera="camera" :play="play" @click="$emit('stop')" :round="true" />
		<div v-if="!play" class="iconContainer flexCenter" @click="$emit('play')">
			<div :class="{ playIcon: iconIsPlay(), recordIcon: iconIsRecord() }">
			</div>
		</div>
		<div v-if="showCameraName" class="name">{{ camera.name }}</div>
	</div>
</template>

<style lang="scss" scoped>
.cameraItem {
	position: relative;
}

.iconContainer {
	position: absolute;
	left: 0px;
	top: 0px;
	width: 100%;
	height: 90%; // HACK! the missing 10% is for SeekBar
	cursor: pointer;
}

.playIcon {
	background-repeat: no-repeat;
	background-size: 30px 30px;
	background-position: center;
	width: 30px;
	height: 30px;
	background-image: url("@/icons/play-circle-outline.svg");
	//filter: invert(1) drop-shadow(1px 1px 3px rgba(0, 0, 0, 0.9));
}

.playIcon:hover {
	filter: invert(1) drop-shadow(0px 0px 1px rgb(183, 184, 255)) drop-shadow(1.5px 1.5px 3px rgba(0, 0, 0, 0.9));
}

.recordIcon {
	background-color: #e00;
	width: 16px;
	height: 16px;
	border-radius: 100px;
	border: solid 2px #fff;
	animation-name: pulse;
	animation-duration: 0.6s;
	animation-iteration-count: infinite;
	animation-direction: alternate;
	animation-timing-function: cubic-bezier(0.1, 0, 0.9, 1); // https://cubic-bezier.com/#0,.2,1,.8
}

@keyframes pulse {
	from {
		transform: scale(1);
		opacity: 1;
	}

	to {
		transform: scale(1.15);
		opacity: 0.5;
	}
}

.name {
	position: absolute;
	right: 4px; // put name on the right, because video-encoded time display is usually on the top left
	top: 4px;
	font-size: 10px;
	color: #fff;
	filter: drop-shadow(0px 0px 2px #000);
	border-radius: 2px;
	padding: 2px 4px;
	background: rgba(0, 0, 0, 0.2)
}
</style>
