<script setup lang="ts">
import type { CameraInfo } from "@/camera/camera";
import JMuxer from "jmuxer";
import { onMounted } from "vue";

let props = defineProps<{
	camera: CameraInfo
}>()

var muxer: JMuxer;
var ws: WebSocket;

function parse(data: ArrayBuffer) {
	var input = new Uint8Array(data);
	var dv = new DataView(input.buffer);

	//var ptsMicro = dv.getBigInt64(0, true);
	let video = input.subarray(0);

	return {
		video: video,
		//duration: duration,
	};
}

function play() {
	var socketURL = "ws://" + window.location.host + "/api/ws/camera/stream/low/" + props.camera.index;
	console.log("Play " + socketURL);
	muxer = new JMuxer({
		node: 'camera' + props.camera.index,
		mode: "video",
		debug: false,
		fps: 10,
		flushingTime: 100,
		//flushingTime: 1000, // we need 1000 for the demo as server provides a chunk data of 1000ms at a time
	});

	ws = new WebSocket(socketURL);
	ws.binaryType = "arraybuffer";
	ws.addEventListener("message", function (event) {
		var data = parse(event.data);
		muxer.feed(data);
	});

	ws.addEventListener("error", function (e) {
		console.log("Socket Error");
	});
}

onMounted(() => {
	play();
})
</script>

<template>
	<div>
		<div> {{ camera.index }} {{ camera.name }} </div>
		<video :id="'camera' + camera.index" controls autoplay />
	</div>
</template>

<style lang="scss" scoped>
</style>