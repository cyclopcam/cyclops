<script setup lang="ts">
import type { CameraInfo } from "@/camera/camera";
import JMuxer from "jmuxer";
import { onMounted, onUnmounted, watch } from "vue";

// Player is for playing a live camera stream.
// A websocket feeds us h264 packets, and we use jmuxer to feed them into
// a <video> object.

let props = defineProps<{
	camera: CameraInfo,
	play: boolean,
	round?: boolean,
	size?: string,
}>()
let emits = defineEmits(['click']);

let muxer: JMuxer | null = null;
let ws: WebSocket | null = null;
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
	let normalDuration = 1000 / props.camera.ld.fps;

	// This is a naive attempt at forcing the player to catch up to realtime, without introducing
	// too much jitter. I'm not sure if it actually works.
	// OK.. interesting.. I left my system on play for a long time (eg 1 hour), and when I came back,
	// the camera was playing daytime, although it was already night time outside. So *somewhere*, we are
	// adding a gigantic buffer. I haven't figured out how to figure out where that is.
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

	let socketURL = "ws://" + window.location.host + "/api/ws/camera/stream/LD/" + props.camera.id;
	console.log("Play " + socketURL);
	muxer = new JMuxer({
		node: 'camera' + props.camera.id,
		mode: "video",
		debug: false,
		// OK.. we want to leave FPS unspecified, so that we can control it per-frame, for backlog catchup
		//fps: 60, // this becomes Max FPS, so.. the speedup during backlog catchup
		maxDelay: 200,
		flushingTime: 100, // jsmuxer basically runs as setInterval(() => flushFrames(), flushingTime)
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
	console.log("Player onClick");
	emits('click');
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
	console.log("Player.vue stop");
	if (ws) {
		ws.close();
		ws = null;
	}
	if (muxer) {
		muxer.destroy();
		muxer = null;
	}
}

function posterURL(): string {
	return "/api/camera/latestImage/" + props.camera.id;
}

function videoStyle(): any {
	/*
	let width = props.camera.ld.width + "px";
	let height = props.camera.ld.height + "px";
	if (props.size) {
		switch (props.size) {
			case "small":
				width = "200px";
				height = "140px";
				break;
			case "medium":
				width = "320px";
				height = "200px";
				break;
			default:
				console.error(`Unknown camera size ${props.size}`);
		}
	}
	*/

	return {
		//width: width,
		//height: height,
		"border-radius": props.round ? "5px" : "",
	}
}

watch(() => props.play, (newVal, oldVal) => {
	if (newVal) {
		play();
	} else {
		stop();
	}
})

onUnmounted(() => {
	stop();
})

onMounted(() => {
	if (props.play)
		play();
})
</script>

<template>
	<video class="video" :id="'camera' + camera.id" autoplay :poster="posterURL()" @play="onPlay" @pause="onPause"
		@click="onClick" :style="videoStyle()" />
</template>

<style lang="scss" scoped>
.video {
	width: 100%;
	height: 100%;
	// This screws up the aspect ratio, but I feel like it's the right UI tradeoff for consistency of the video widgets.
	// Without this, on Chrome on Linux, as soon as the player starts decoding frames, it adjusts itself to the actual
	// aspect ratio of the decoded video stream, and this usually leaves a letter box in our UI. Normally I hate distorting
	// aspect ratio, but in this case I actually think it's the best option.
	object-fit: fill;
}
</style>