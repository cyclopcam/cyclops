<script setup lang="ts">
import Buttin from "../core/Buttin.vue";
import RedDot from "@/icons/red-dot.svg";
import { fetchRecordings, Recording } from "@/recording/recording";
import { onMounted, ref } from "vue";
import RecordingItem from "./RecordingItem.vue";

let networkError = ref('');
let recordings = ref([] as Recording[]);

async function getRecordings() {
	let r = await fetchRecordings();
	if (!r.ok) {
		networkError.value = r.err;
		return;
	}
	networkError.value = '';
	recordings.value = r.value;
}

function showHelp(): boolean {
	return recordings.value.length === 0;
}

onMounted(() => {
	getRecordings();
})
</script>

<template>
	<div class="train flexColumnCenter">
		<div v-if="networkError" class="error">{{ networkError }}</div>
		<p v-if="showHelp()" class="helpTopic">Train your system by recording videos that simulate alarm conditions.</p>
		<div style="height: 20px" />
		<buttin :icon="RedDot" iconSize="16px">New Recording</buttin>
		<div class="recordings">
			<recording-item v-for="rec of recordings" :recording="rec" />
		</div>
	</div>
</template>

<style lang="scss" scoped>
.train {
	//background-color: cornsilk;
	margin: 25px 10px 10px 10px;
}

.recordings {
	display: flex;
	flex-wrap: wrap;
	gap: 10px;
	margin: 30px 20px 10px 20px;
	justify-content: center;
}
</style>
