<script setup lang="ts">
import Buttin from "@/components/core/Buttin.vue";
import { globals } from "@/globals";
import { fetchRecordings, Recording } from "@/recording/recording";
import { onMounted, ref } from "vue";
import router from "@/router/routes";
import PlusCircle from "@/icons/plus-circle.svg";
import RecordingItem from "./RecordingItem.vue";

let emits = defineEmits(['recordNew']);

let recordings = ref([] as Recording[]);

function showHelp(): boolean {
	return recordings.value.length === 0;
}

async function getRecordings() {
	let r = await fetchRecordings();
	if (!r.ok) {
		globals.networkError = r.err;
		return;
	}
	globals.networkError = '';
	recordings.value = r.value;
}

onMounted(() => {
	getRecordings();
})

</script>

<template>
	<div class="flexColumnCenter">
		<div v-if="showHelp()" class="helpTopic">Train your system by recording videos that simulate alarm conditions.
		</div>
		<!--
		<buttin :icon="PlusCircle" iconSize="16px" @click="onRecordNew">New Recording
		</buttin>
		<div class="groupLabel">Recordings</div>
		-->
		<div class="recordings">
			<recording-item @click="$emit('recordNew')" />
			<recording-item v-for="rec of recordings" :recording="rec" />
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
