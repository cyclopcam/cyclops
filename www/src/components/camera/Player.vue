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
let backlogDone = false;
let nPackets = 0;
let nBytes = 0;
let firstPacketTime = 0;

function parse(data: ArrayBuffer) {
	let input = new Uint8Array(data);
	let dv = new DataView(input.buffer);

	let now = new Date().getTime();
	if (nPackets === 0) {
		firstPacketTime = now;
	}

	//let foo1 = dv.getUint32(0, true);
	//let foo2 = dv.getUint32(4, true);
	//console.log("foos", foo1, foo2);
	//let pts = dv.getFloat64(0, true);
	let flags = dv.getUint32(0, true);
	let backlog = (flags & 1) !== 0;
	//console.log("pts", pts);
	let video = input.subarray(4);

	nBytes += input.length;
	nPackets++;

	if (!backlog && !backlogDone) {
		let bytesPerSecond = 1000 * nBytes / (now - firstPacketTime);
		console.log(`backlogDone in ${now - firstPacketTime} ms. ${nBytes} bytes over ${nPackets} packets which is ${bytesPerSecond} bytes/second`);
		backlogDone = true;
	}

	// It is better to inject a little bit of frame duration (as opposed to leaving it undefined),
	// because it reduces the jerkiness of the video that we see, presumably due to network and/or camera jitter
	let normalDuration = 1000 / props.camera.low.fps;

	// This is a naive attempt at forcing the player to catch up to realtime, without introducing
	// too much jitter. I'm not sure if it actually works.
	normalDuration *= 0.9;

	// during backlog catchup, we leave duration undefined, which causes the player to catch up
	// as fast as it can (which is precisely what we want).

	return {
		video: video,
		duration: backlog ? undefined : normalDuration,
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
		// OK.. we want to leave FPS unspecified, so that we can control it per-frame, for backlog catchup
		//fps: 60, // this becomes Max FPS, so.. the speedup during backlog catchup
		maxDelay: 200,
		flushingTime: 100, // jsmuxer basically runs with setInterval(flushFrames, flushingTime)
	} as any);

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
	if (!isPlaying()) {
		//isFirstPlay = false;
		play();
	} else if (isPlaying()) {
		stop();
	}
}

function onPlay() {
	console.log("onPlay");
	//play();
}

function onPause() {
	console.log("onPause");
	//stop();
}

function stop() {
	console.log("stop");
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
		if (!isPlaying()) {
			play();
		}
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
		<video class="video" :id="'camera' + camera.id" autoplay :poster="posterURL()" @play="onPlay" @pause="onPause"
			@click="onClick" :style="videoStyle()" />
		<button @click="onRecordStartStop">{{ isRecording ? "stop" : "record" }}</button>
	</div>
</template>

<style lang="scss" scoped>
// Can't figure out why video resizing in Chrome desktop.. this happens on linux only, so just gonna ignore it.
// Curiously, the linux resized framebuffer seems like it has the correct aspect ratio.
//.video {
//	object-fit: fit;
//}
</style>