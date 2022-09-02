<script setup lang="ts">
import { globals } from "@/globals";
import { fetchRecordings, Recording } from "@/recording/recording";
import { onMounted, reactive, ref } from "vue";
import RecordingItem from "./RecordingItem.vue";
import Buttin from "../../core/Buttin.vue";
import Trash from '@/icons/trash-2.svg';
import Labeler from './Labeler.vue';
import LabelerDialog from './LabelerDialog.vue';
import path from "path";
import router from "@/router/routes";

let emits = defineEmits(['recordNew']);

let haveRecordings = ref(false);
let recordings = ref([] as Recording[]);
let playerCookie = ref(''); // ensures that only one RecordingItem is playing at a time
let selection = reactive(new Set<number>()); // Every update will probably cause all RecordingItems to get re-rendered, so this might not scale well
let labelRecording = ref(null as Recording | null);

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

function onOpenLabeler(rec: Recording) {
	labelRecording.value = rec;
	//router.push({ name: 'rtTrainLabelRecording', params: { recordingID: rec.id } });
}

onMounted(() => {
	getRecordings();
})

</script>

<template>
	<div class="flexColumnCenter manageRoot">
		<!-- <div class="stepLabel">Edit Recordings</div> -->
		<div>
			<buttin :icon="Trash" icon-size="14px" :disabled="selection.size === 0">Delete</buttin>
		</div>

		<div class="recordings">
			<!--
			<recording-item :player-cookie="playerCookie" @click="$emit('recordNew')" />
			-->
			<recording-item v-for="rec of recordings" :player-cookie="playerCookie" :recording="rec"
				:selection="selection" @play-inline="onPlayInline" @delete="onDelete(rec)"
				@open-labeler="onOpenLabeler(rec)" />
		</div>

		<labeler-dialog v-if="labelRecording" :initial-recording="labelRecording" @close="labelRecording = null" />
	</div>
</template>

<style lang="scss" scoped>
@import '@/assets/vars.scss';

.manageRoot {
	margin: 25px 10px 10px 10px;
}

.recordings {
	display: flex;
	flex-wrap: wrap;
	gap: 30px;
	margin: 20px 20px 10px 20px;
	justify-content: center;
	max-width: 1400px;
	//background-color: antiquewhite;

	@media (max-width: $mobileCutoff) {
		// fit two thumbnails per row (more styles for this in RecordingItem.vue)
		//margin: 10px 5px 10px 5px;
		max-width: 98vw;
		gap: 12px;
	}
}
</style>
