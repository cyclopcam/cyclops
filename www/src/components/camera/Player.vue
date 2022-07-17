<script setup lang="ts">
import type { CameraInfo } from "@/camera/camera";
import JMuxer from "jmuxer";
import { onMounted, reactive, ref } from "vue";

let props = defineProps<{
	camera: CameraInfo
}>()

let muxer: JMuxer | null = null;
let ws: WebSocket | null = null;
let isFirstPlay = true;
let isRecording = ref(false);

function parse(data: ArrayBuffer) {
	let input = new Uint8Array(data);
	let dv = new DataView(input.buffer);

	//var ptsMicro = dv.getBigInt64(0, true);
	let video = input.subarray(0);

	return {
		video: video,
		//duration: duration,
	};
}

function isPlaying(): boolean {
	return muxer !== null;
}

function play() {
	console.log("play(). isPlaying: " + (isPlaying() ? "yes" : "no"));
	if (isPlaying())
		return;

	let socketURL = "ws://" + window.location.host + "/api/ws/camera/stream/low/" + props.camera.id;
	console.log("Play " + socketURL);
	muxer = new JMuxer({
		node: 'camera' + props.camera.id,
		mode: "video",
		debug: false,
		fps: 10,
		flushingTime: 100,
		//flushingTime: 1000, // we need 1000 for the demo as server provides a chunk data of 1000ms at a time (original comment from jmuxer sample code)
	});

	ws = new WebSocket(socketURL);
	ws.binaryType = "arraybuffer";
	ws.addEventListener("message", function (event) {
		if (muxer) {
			let data = parse(event.data);
			muxer.feed(data);
		}
	});

	ws.addEventListener("error", function (e) {
		console.log("Socket Error");
	});
}

function onClick() {
	console.log("onClick");
	if (isFirstPlay) {
		isFirstPlay = false;
		play();
	}
}

function onPlay() {
	console.log("onPlay");
	play();
}

function onPause() {
	console.log("onPause");
	if (ws) {
		ws.close();
		ws = null;
	}
	if (muxer) {
		muxer.destroy();
		muxer = null;
	}
}

async function onRecordStartStop() {
	if (isRecording.value) {
		await fetch("/api/record/stop", { method: "POST" });
		isRecording.value = false;
	} else {
		await fetch("/api/record/start/" + props.camera.id, { method: "POST" })
		isRecording.value = true;
	}
}

function posterURL(): string {
	return "/api/camera/latestImage/" + props.camera.id;
}

function videoStyle(): any {
	return {
		width: props.camera.low.width + "px",
		height: props.camera.low.height + "px",
	}
}

//onMounted(() => {
//	play();
//})
</script>

<template>
	<div>
		<div> {{ camera.id }} {{ camera.name }} </div>
		<video :id="'camera' + camera.id" autoplay :poster="posterURL()" @play="onPlay" @pause="onPause"
			@click="onClick" :style="videoStyle()" />
		<button @click="onRecordStartStop">{{ isRecording ? "stop" : "record" }}</button>
	</div>
</template>

<style lang="scss" scoped>
</style>