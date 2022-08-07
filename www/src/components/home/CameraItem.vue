<script setup lang="ts">
import type { CameraInfo } from '@/camera/camera.js';
import Player from '../camera/Player.vue';

let props = defineProps<{
	camera: CameraInfo,
	play: boolean,
	size?: string,
	icon?: string, // 'play', 'record' (default = play)
}>()
defineEmits(['play', 'stop']);

function style(): any {
	// We want an aspect ratio that is most average, because in <player> we distort the aspect ratio
	// We use aspect = 1.5 because it's more square than 16:9 (1.777), to accomodate cameras that are more square.
	let aspect = 1.5;
	let width = 320;
	if (props.size) {
		switch (props.size) {
			case "small":
				width = 200;
				break;
			case "medium":
				// this is the default
				break;
			default:
				let n = parseFloat(props.size);
				if (n !== 0) {
					width = n;
				} else {
					console.error(`Unknown camera size ${props.size}`);
				}
		}
	}
	return {
		width: width + 'px',
		height: Math.round(width / aspect) + 'px',
	};
}
function iconIsPlay() { return (props.icon ?? "play") === "play"; }
function iconIsRecord() { return (props.icon ?? "play") === "record"; }

</script>

<template>
	<div class="flex cameraItem" :style="style()">
		<player :camera="camera" :play="play" @click="$emit('stop')" :round="true" />
		<div v-if="!play" :class="{ icon: true, playIcon: iconIsPlay(), recordIcon: iconIsRecord(), }"
			@click="$emit('play')"></div>
		<div class="name">{{ camera.name }}</div>
	</div>
</template>

<style lang="scss" scoped>
.cameraItem {
	position: relative;
}

.icon {
	position: absolute;
	left: 0px;
	top: 0px;
	background-repeat: no-repeat;
	background-size: 50px 50px;
	background-position: center;
	width: 100%;
	height: 100%;
	cursor: pointer;
}

.playIcon {
	background-image: url("@/icons/play-circle.svg");
	filter: invert(1) drop-shadow(1px 1px 3px rgba(0, 0, 0, 0.9));
}

.playIcon:hover {
	filter: invert(1) drop-shadow(0px 0px 1px rgb(183, 184, 255)) drop-shadow(1.5px 1.5px 3px rgba(0, 0, 0, 0.9));
}

.recordIcon {
	background-image: url("@/icons/red-dot.svg");
	background-size: 30px 30px;
	filter: drop-shadow(0px 0px 3px rgba(0, 0, 0, 0.5));
}

.name {
	position: absolute;
	left: 4px;
	top: 4px;
	font-size: 10px;
	color: #fff;
	filter: drop-shadow(0px 0px 2px #000);
	border-radius: 2px;
	padding: 2px 4px;
	background: rgba(0, 0, 0, 0.2)
}
</style>
