<script setup lang="ts">
import { fetchEvent, type SystemEvent } from '@/events/event';
import Modal from '@/components/widgets/Modal.vue';
import { onMounted, ref } from 'vue';
import { globals } from '@/globals';

let props = defineProps<{
	notificationId: number
}>()
let emits = defineEmits(['close']);

let notification = ref(null as SystemEvent | null);
let error = ref("");

function title(): string {
	let n = notification.value;
	if (!n) return "...";
	if (n.eventType === "arm") {
		return `Armed by ${n.detail.arm!.userId}`;
	} else if (n.eventType === "disarm") {
		return `Disarmed by ${n.detail.arm!.userId}`;
	} else if (n.eventType === "alarm") {
		if (n.detail.alarm?.alarmType === "camera-object") {
			return `Intruder detected`;
		} else if (n.detail.alarm?.alarmType === "panic") {
			return `Panic Button Pressed`;
		} else {
			return `Unknown alarm event`;
		}
	} else {
		return `Notification: ${n.id}`;
	}
}

function showImage(): boolean {
	return notification.value?.eventType === "alarm" && notification.value?.detail.alarm?.alarmType === "camera-object";
}

function imageSrc(): string {
	let n = notification.value!;
	return `/api/events/${n.id}/image`;
}

async function fetchNotification() {
	let res = await fetchEvent(props.notificationId);
	if (res) {
		notification.value = res;
	} else {
		if (globals.networkError === "Not Found") {
			error.value = "Notification not found";
		} else {
			error.value = globals.networkError || "Failed to fetch notification";
		}
	}
}

function onImageError() {
	error.value = "Failed to load video image";
}

function onDismiss() {
	emits('close');
}

onMounted(() => {
	fetchNotification();
});

</script>

<template>
	<modal v-if="notificationId !== 0" @close="emits('close')" tint="rgba(0,0,0,0.75)">
		<div class="container">
			<div v-if="notification">
				<div class="title">{{ title() }}</div>
				<div v-if="showImage()" class="imageContainer">
					<img :src="imageSrc()" alt="Alarm Image" @error="onImageError" />
				</div>
			</div>
			<div v-else-if="error === ''" class="loading">
				Loading details...
			</div>

			<div v-if="error" class="error">
				{{ error }}
			</div>
			<div class="bottom">
				<button @click="onDismiss">Dismiss</button>
			</div>
		</div>
	</modal>
</template>

<style lang="scss" scoped>
.container {
	padding: 10px;
	background-color: #333;
	display: flex;
	flex-direction: column;
	justify-content: center;
}

.title {
	color: rgb(252, 79, 4);
	font-weight: bold;
	padding: 10px;
	font-size: 20px;
	display: flex;
	align-content: center;
	justify-content: center;
}

.imageContainer {
	margin-top: 10px;
	display: flex;
	justify-content: center;
}

img {
	max-width: 100%;
	border-radius: 5px;
}

.error {
	margin-top: 10px;
	color: red;
}

.bottom {
	margin-top: 20px;
	margin-bottom: 10px;
	display: flex;
	justify-content: center;
}
</style>
