<script setup lang="ts">
import { CameraInfo } from '@/camera/camera';
import { globals } from '@/globals';
import CameraItem from '@/components/home/CameraItem.vue';
import { ref, onMounted } from 'vue';
import RedDot from "@/icons/red-dot.svg";
import Stop from "@/icons/stop.svg";
import Buttin from "../../core/Buttin.vue";
import { fetchOrErr } from '@/util/util';
import RecordingItem from './RecordingItem.vue';
import { Ontology, Recording } from '@/recording/recording';

let minRecordingSeconds = 5;
let maxRecordingSeconds = 45; // SYNC-MAX-TRAIN-RECORD-TIME

enum States {
	PickCamera,
	PreRecord,
	Recording,
	PostRecord,
}

let nRecordings = ref(0);
let state = ref(States.PickCamera);
let camera = ref(new CameraInfo());
let playLive = ref(false);
let startedAt = ref(new Date());
let timeNow = ref(new Date());
let startError = ref("");
let recorderID = 0;
let newRecording = ref(new Recording());
let ontologies = ref([] as Ontology[]);
let latestOntology = ref(new Ontology());

function cameras(): CameraInfo[] {
	return globals.cameras;
}

function onPlay(cam: CameraInfo) {
	state.value = States.PreRecord;
	camera.value = cam;
	playLive.value = true;
	// skip straight ahead to recording
	startRecording();
}

async function startRecording() {
	let r = await fetchOrErr("/api/record/start/" + camera.value.id, { method: "POST" });
	if (!r.ok) {
		globals.networkError = r.error;
		return;
	}
	recorderID = parseInt(await r.r.text());
	state.value = States.Recording;
	startedAt.value = new Date();
	timeTicker();
}

async function stopRecording() {
	let r = await fetchOrErr("/api/record/stop/" + recorderID, { method: "POST" });
	if (!r.ok) {
		globals.networkError = r.error;
		return;
	}
	newRecording.value = Recording.fromJSON(await r.r.json());
	state.value = States.PostRecord;
	playLive.value = false;
}

function saveRecording() {
	state.value = States.PreRecord;
	nRecordings.value++;
}

async function discardRecording() {
	let r = await fetchOrErr("/api/record/delete/" + newRecording.value.id, { method: "POST" });
	if (!r.ok) {
		globals.networkError = r.error;
		return;
	}
	state.value = States.PreRecord;
}

function timeTicker() {
	if (state.value !== States.Recording) {
		return;
	}
	timeNow.value = new Date();
	setTimeout(timeTicker, 200);
}

function seconds(): number {
	return (timeNow.value.getTime() - startedAt.value.getTime()) / 1000;
}

function status(): string {
	let elapsed = seconds();
	return `${Math.round(elapsed)} seconds`;
}

onMounted(async () => {
	await Ontology.fetchIntoReactive(ontologies, latestOntology);
})

</script>

<template>
	<div class="recorderRoot">
		<div v-if="state === States.PickCamera" class="flexColumnCenter">
			<div class="stepLabel">Record</div>
			<div class="flex picker">
				<camera-item v-for="cam of cameras()" :camera="cam" icon="record" :play="false" size="220"
					@play="onPlay(cam)" class="cameraItemPicker" />
			</div>
			<div style="margin: 15px; font-size: 14px">
				Recordings can be anywhere from {{
					minRecordingSeconds
				}}
				to {{ maxRecordingSeconds }} seconds long.
			</div>
		</div>
		<div v-else class="flexColumnCenter">
			<camera-item v-if="state === States.PreRecord || state === States.Recording" :camera="camera"
				:play="playLive" size="280" />
			<div style="height: 10px" />
			<div v-if="state === States.PreRecord" class="flexColumnCenter recordingBlock">
				<div class="stepHint hints">
					<ul>
						<li>
							Create a video of yourself performing a suspicious activity, or anything else
							that you want the system to learn.
						</li>
						<li>Recordings can be anywhere from {{
							minRecordingSeconds
						}}
							to {{ maxRecordingSeconds }} seconds long.
						</li>
					</ul>
				</div>
				<div v-if="startError" class="stepHint error" style="text-align:center">{{ startError }}</div>
				<buttin :icon="RedDot" iconSize="16px" @click="startRecording()">
					{{ nRecordings === 0 ? 'Start Recording' : 'Record Another' }}
				</buttin>
			</div>
			<div v-else-if="state === States.Recording" class="flexColumnCenter recordingBlock">
				<div class="status">{{ status() }}</div>
				<progress :value="seconds()" :max="maxRecordingSeconds" class="progress" />
				<buttin :icon="Stop" iconSize="16px" @click="stopRecording()">Stop
				</buttin>
			</div>
			<div v-else-if="state === States.PostRecord" class="flexColumnCenter recordingBlock">
				<recording-item v-if="newRecording.id !== 0" player-cookie="12345" :recording="newRecording"
					:ontologies="ontologies" :play-at-startup="true" />
				<div class="flex saveOrDiscard">
					<buttin @click="discardRecording()" :danger="true">Discard</buttin>
					<div class="dangerSpacer" />
					<buttin @click="saveRecording()" :focal="true">Save</buttin>
				</div>
			</div>
		</div>
	</div>
</template>

<style lang="scss" scoped>
.recorderRoot {
	margin: 25px 10px 10px 10px;
}

.picker {
	gap: 10px;
	flex-wrap: wrap;
	justify-content: center;
	margin: 20px 20px 16px 20px;
}

.cameraItemPicker {
	border: solid 2px rgba(0, 0, 0, 0);
	border-radius: 7px;
}

.cameraItemPicker:hover {
	border: solid 2px rgb(72, 144, 232);
}

.recordingBlock {
	margin: 5px 0 5px 0;
}

.status {
	margin: 5px 0 0px 0;
}

.progress {
	width: 240px;
	height: 30px;
	margin: 2px 0 20px 0;
}

.saveOrDiscard {
	margin: 20px 0 0 0;
}

.hints {
	margin: 0 0 10px 0;
}

ul {
	padding-left: 20px;
	padding-right: 5px;
}

li {
	margin: 10px 0;
}

.error {
	color: #d00;
}
</style>
