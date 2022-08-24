<script setup lang="ts">
import { globals } from "@/globals";
import { fetchRecordings, Recording } from "@/recording/recording";
import { onMounted, ref } from "vue";
import RecordingItem from "./RecordingItem.vue";

let emits = defineEmits(['recordNew']);

let haveRecordings = ref(false);
let recordings = ref([] as Recording[]);
let playerCookie = ref(''); // ensures that only one RecordingItem is playing at a time

async function getRecordings() {
	let r = await fetchRecordings();
	if (!r.ok) {
		globals.networkError = r.err;
		return;
	}
	globals.networkError = '';
	r.value.sort((a, b) => b.startTime.getTime() - a.startTime.getTime());
	recordings.value = r.value;
	haveRecordings.value = true;
}

function onPlayInline(cookie: string) {
	playerCookie.value = cookie;
}

function onDelete(rec: Recording) {
	let idx = recordings.value.indexOf(rec);
	if (idx !== -1) {
		recordings.value.splice(idx, 1);
	}
}

onMounted(() => {
	getRecordings();
})

</script>

<template>
	<div class="flexColumnCenter">
		<!--
		<buttin :icon="PlusCircle" iconSize="16px" @click="onRecordNew">New Recording
		</buttin>
		<div class="groupLabel">Recordings</div>
		-->
		<div class="recordings">
			<!--
			<recording-item :player-cookie="playerCookie" @click="$emit('recordNew')" />
			-->
			<recording-item v-for="rec of recordings" :player-cookie="playerCookie" :recording="rec"
				@play-inline="onPlayInline" @delete="onDelete(rec)" />
		</div>
	</div>
</template>

<style lang="scss" scoped>
@import '@/assets/vars.scss';

.recordings {
	display: flex;
	flex-wrap: wrap;
	gap: 10px;
	margin: 10px 20px 10px 20px;
	justify-content: center;
	max-width: 90vw;
	//background-color: antiquewhite;

	@media (max-width: $mobileCutoff) {
		// fit two thumbnails per row (more styles for this in RecordingItem.vue)
		margin: 10px 5px 10px 5px;
		max-width: 98vw;
		gap: 8px;
	}
}
</style>
