<script setup lang="ts">
import { globals } from "@/globals";
import { Ontology, Recording } from "@/recording/recording";
import { onMounted, reactive, ref, watch } from "vue";
import RecordingItem from "./RecordingItem.vue";
import Buttin from "../../core/Buttin.vue";
import Trash from '@/icons/trash-2.svg';
import LabelerDialog from './LabelerDialog.vue';
import { pushRoute } from "@/router/routes";

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
	haveRecordings.value = true;
}

async function getRecording(id: number): Promise<Recording | null> {
	let r = await Recording.fetch(id);
	if (!r.ok) {
		globals.networkError = r.err;
		return null;
	}
	globals.networkError = '';
	return r.value;
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
			// just navigate to manage page, to avoid useless paths in our history
			pushRoute({ name: 'rtTrainEditRecordings' });
		}
	} else {
		console.log("labelRecording.value = null");
		labelRecording.value = null;
	}
}

watch(() => props.id, (newVal) => {
	console.log("watch changed", newVal);
	let id = parseInt(newVal ?? '');
	onLabelIDChanged(id);
	if (!haveRecordings.value) {
		// this path is when you navigate to an invalid item, eg http://mars:3000/train/edit/999999
		globals.networkError = '';
		getRecordings();
	}
})

onMounted(async () => {
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
			<!--
			<recording-item :player-cookie="playerCookie" @click="$emit('recordNew')" />
			-->
			<recording-item v-for="rec of recordings" :player-cookie="playerCookie" :recording="rec"
				:selection="selection" @play-inline="onPlayInline" @delete="onDelete(rec)"
				@open-labeler="onOpenLabeler(rec)" />
		</div>

		<labeler-dialog v-if="labelRecording" :initial-recording="labelRecording" :ontologies="ontologies"
			:latest-ontology="latestOntology" @close="labelRecording = null" />
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
