<script setup lang="ts">
import type { CameraRecord } from '@/db/config/configdb';
import { encodeQuery } from '@/util/util';
import { computed } from '@vue/reactivity';
import { onMounted, onUnmounted, ref } from 'vue';
import Modal from '../widgets/Modal.vue';

let props = defineProps<{
	camera: CameraRecord,
}>()
let emits = defineEmits(['close']);

enum States {
	Testing,
	Error,
	Success
}

let preview = ref(null);
let status = ref("Initializing");
let state = ref(States.Testing);
let isShrinking = ref(false);
let ws: WebSocket | null = null;
let imageBlob: Blob | null = null;

interface ServerMessage {
	error: string;
	status: string;
	image: string;
}

let isTesting = computed(() => state.value === States.Testing);
let isError = computed(() => state.value === States.Error);
let isSuccess = computed(() => state.value === States.Success);

function onClose() {
	if (isError.value) {
		emits('close', { error: status.value });
	} else if (isSuccess.value) {
		emits('close', { image: imageBlob });
	} else {
		emits('close', { error: "Cancelled" });
	}
}

function shrinkAndClose() {
	isShrinking.value = true;
	setTimeout(() => {
		onClose();
	}, 250); // keep this timeout in sync with the animation duration in the CSS
}

onMounted(() => {

	let scheme = window.location.origin.startsWith("https") ? "wss://" : "ws://";
	//let socketURL = scheme + window.location.host + "/api/ws/config/testCamera?" + encodeQuery(bearerTokenQuery());
	let socketURL = scheme + window.location.host + "/api/ws/config/testCamera";

	ws = new WebSocket(socketURL);
	//ws.binaryType = "arraybuffer";

	ws.addEventListener("open", function (event) {
		// send camera details, because we can't send POST data with a websocket
		ws?.send(JSON.stringify(props.camera.toJSON()));
	});

	ws.addEventListener("message", function (event) {
		if (typeof event.data === "string") {
			//console.log("string message", event);
			let msg = JSON.parse(event.data) as ServerMessage;
			if (msg.error) {
				state.value = States.Error;
				status.value = msg.error;
				//setTimeout(onClose, 200);
				setTimeout(shrinkAndClose, 300);
			} else if (msg.status) {
				status.value = msg.status;
			}
		} else if (typeof event.data === "object") {
			// A binary message means we have a decoded test image from the camera
			//console.log("binary message", event);
			imageBlob = event.data;
			state.value = States.Success;
			status.value = "Success!";
			let url = window.URL.createObjectURL(event.data);
			if (preview.value) {
				(preview.value as HTMLImageElement).src = url;
			}
			setTimeout(shrinkAndClose, 300);
		}
	});

	ws.addEventListener("error", function (e) {
		console.log("Socket Error");
	});


})

onUnmounted(() => {
	if (ws) {
		ws.close();
		ws = null;
	}
})

</script>

<template>
	<modal tint="mild">
		<div :class="{ smallDialog: true, flexColumnCenter: true, shrinkModal: isShrinking }">
			<h4 style="margin-top: 5px">Testing Camera Connection</h4>
			<!--
			<div class="preview flexCenter shadow5L">
				<img ref="previewImage" style="width: 100%; height: 100%" />
			</div>
			-->
			<img ref="preview" class="preview shadow5L" />
			<div :class="{ status: true, error: isError, success: isSuccess }">{{ status }}</div>
			<div class="flex" style="justify-content: flex-end">
				<button @click="onClose">{{ isTesting ? 'Cancel' : 'Close' }}</button>
			</div>
		</div>
	</modal>
</template>

<style lang="scss" scoped>
.container {
	padding: 20px;
}

@keyframes shrink {
	0% {
		transform: scale(1);
	}

	100% {
		transform: scale(0);
	}
}

.shrinkModal {
	// keep this duration in sync with the timeout in the shrinkAndClose() function
	animation: shrink 250ms ease-in-out forwards;
}

// I can't get rid of the border around this image!!! never seen this before....
.preview {
	width: 260px;
	min-height: 160px;
	border-radius: 3px;
}

.status {
	text-align: center;
	width: 300px;
	font-size: 14px;
	margin: 20px 0px;
}

.error {
	font-size: 16px;
	color: #d00;
}

.success {
	font-size: 18px;
	font-weight: 600;
	color: #0a0;
}
</style>
