<script setup lang="ts">
import { globals } from "@/globals";
import { Ontology, Recording } from "@/recording/recording";
import { onMounted, reactive, ref, watch } from "vue";
import RecordingItem from "./RecordingItem.vue";
import Buttin from "../../core/Buttin.vue";
import Trash from '@/icons/trash-2.svg';
import LabelerDialog from './LabelerDialog.vue';
import { pushRoute } from "@/router/routes";

// NOTE: The state control of EditRecordings is a bit tricky, because it basically has two modes:
// 1. Edit recordings
// 2. Label <a specific> recording
// These two states are reflected in the two routes rtTrainEditRecordings and rtTrainLabelRecording.
// We need to be careful to keep all of this in sync. For example, when the user finishes labelling
// of a recording, then we must change our route from /label/123 to /edit.
// My initial design was to make the 'label' mode a separate top-level page, but that was quite
// unpleasant, due to code duplication, so that's why we've got this hybrid page.
// There's probably a cleaner way to do this.

let props = defineProps<{
	id?: string, // ID of video to edit (comes in via route)
}>()

let emits = defineEmits(['recordNew']);

let haveRecordings = ref(false);
let recordings = ref([] as Recording[]);
let playerCookie = ref(''); // ensures that only one RecordingItem is playing at a time
let selection = reactive(new Set<number>()); // Every update will probably cause all RecordingItems to get re-rendered, so this might not scale well
let labelRecording = ref(null as Recording | null); // Recording that we are busy labelling
let latestOntology = ref(new Ontology());
let ontologies = ref([] as Ontology[]);

async function getRecordings() {
	let r = await Recording.fetchAll();
	if (!r.ok) {
		globals.networkError = r.err;
		return;
	}
	globals.networkError = '';
	r.value.sort((a, b) => b.startTime.getTime() - a.startTime.getTime());
	recordings.value = r.value;
	recordings.value = r.value.map(x => reactive(x));
	haveRecordings.value = true;
}

async function getRecording(id: number): Promise<Recording | null> {
	let r = await Recording.fetch(id);
	if (!r.ok) {
		globals.networkError = r.err;
		return null;
	}
	globals.networkError = '';
	return reactive(r.value);
}

async function getOntologies() {
	let r = await Ontology.fetch();
	if (!r.ok) {
		globals.networkError = r.err;
		return;
	}
	ontologies.value = r.value;
	// We expect the server to ensure that there's always at least one ontology record
	let latest = Ontology.latest(ontologies.value);
	if (latest) {
		latestOntology.value = latest;
	}
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
	pushRoute({ name: 'rtTrainLabelRecording', params: { id: rec.id } });
}

async function onLabelIDChanged(id: number) {
	if (id) {
		let rec = await getRecording(id);
		if (rec) {
			labelRecording.value = rec;
		} else {
			// just navigate to management page, to avoid useless paths in our history
			labelRecording.value = null;
			pushRoute({ name: 'rtTrainEditRecordings' });
		}
	} else {
		//console.log("labelRecording.value = null");
		labelRecording.value = null;
	}
}

function onEditFinished() {
	// Update the Recording object inside 'recordings', with the latest edited Recording object
	let idx = recordings.value.findIndex(x => x.id === labelRecording.value!.id);
	recordings.value[idx] = labelRecording.value!;
	labelRecording.value = null;
	pushRoute({ name: 'rtTrainEditRecordings' });
}

watch(() => props.id, (newVal) => {
	//console.log("watch changed props.id", newVal);
	let id = parseInt(newVal ?? '');
	onLabelIDChanged(id);
	if (!haveRecordings.value) {
		// this path is when you navigate to an invalid item, eg http://mars:3000/train/edit/999999
		globals.networkError = '';
		getRecordings();
	}
})

onMounted(async () => {
	//console.log("EditRecordings onMounted");

	await getOntologies();

	if (props.id) {
		await onLabelIDChanged(parseInt(props.id));
	} else {
		await getRecordings();
	}
})

</script>

<template>
	<div class="flexColumnCenter manageRoot">
		<!-- <div class="stepLabel">Edit Recordings</div> -->
		<div>
			<buttin :icon="Trash" icon-size="14px" :disabled="selection.size === 0">Delete</buttin>
		</div>

		<div class="recordings">
			<recording-item v-for="rec of recordings" :player-cookie="playerCookie" :recording="rec"
				:ontologies="ontologies" :selection="selection" @play-inline="onPlayInline" @delete="onDelete(rec)"
				@open-labeler="onOpenLabeler(rec)" />
		</div>

		<labeler-dialog v-if="labelRecording" :initial-recording="labelRecording" :ontologies="ontologies"
			:latest-ontology="latestOntology" @close="onEditFinished" />
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
