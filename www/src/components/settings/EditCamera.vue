<script setup lang="ts">
import { CameraRecord } from '@/db/config/configdb';
import { computed, onBeforeUnmount, onMounted, ref, nextTick } from 'vue';
import { constants } from '@/constants';
import CameraTester from './CameraTester.vue';
import CameraPreview from './CameraPreview.vue';
import type { CameraTestResult } from './config';
import { fetchOrErr, sleep } from '@/util/util';
import WideRoot from '@/components/widewidgets/WideRoot.vue';
import WideInput from '@/components/widewidgets/WideInput.vue';
import WideButton from '@/components/widewidgets/WideButton.vue';
import WideSection from '@/components/widewidgets/WideSection.vue';
import Confirm from '@/components/widgets/Confirm.vue';
import { useRouter } from 'vue-router';
import { pushRoute } from "@/router/helpers";
import { globals } from '@/globals';
//import { ScrollMemory } from '@/util/scroll';

let props = defineProps<{
	id: string, // either the ID or "new"
}>();

let router = useRouter();

//let scrollSaver = new ScrollMemory("settingsRoot");

enum TestResult {
	Unknown,
	Fail,
	Success,
}

// If we're editing an existing camera, then 'original' was the camera at the time we opened the dialog
let original = ref(new CameraRecord());

// The configuration that was last tested and connection succeeded
let lastGoodConfig = ref(new CameraRecord());

// After a successful test, this will be the image blob
let testResultImageBlob = ref(null as Blob | null);

let testBusy = ref(false);
let testResult = ref(TestResult.Unknown);
let busySaving = ref(false);
let showConfirmUnpair = ref(false);
let unpairBusy = ref(false);
let error = ref('');

let returnRoute = 'rtSettingsHome';

// This is useful for development on the Edit Camera workflow, because it allows you
// to change a trivial field and then go through the Test Camera.. Save Changes process.
let nameChangeNeedsTest = false;

// The data from 'original' is all blank at this stage, but by making copies of each item, we get the same type inside the ref.
let host = ref(original.value.host);
let username = ref(original.value.username);
let password = ref(original.value.password);
let model = ref(original.value.model);
let name = ref(original.value.name);

let isNewCamera = computed(() => props.id === 'new');

function areCameraConnectionsEqual(a: CameraRecord, b: CameraRecord): boolean {
	if (nameChangeNeedsTest && a.name != b.name)
		return false;

	//console.log(`a.host: ${a.host}, b.host: ${b.host}`);

	return a.host == b.host &&
		a.username == b.username &&
		a.password == b.password &&
		a.model == b.model;
}

function onNameChanged() {
	if (!isNewCamera.value) {
		onSave(false);
	}
}

function needTest(): boolean {
	if (testResult.value === TestResult.Unknown || testResult.value === TestResult.Fail)
		return true;

	return !areCameraConnectionsEqual(lastGoodConfig.value, newCameraRecordFromLocalState());
}

function isTestDisabled(): boolean {
	return (host.value == '' || model.value == '' || username.value == '' || password.value == '') ||
		areCameraConnectionsEqual(lastGoodConfig.value, newCameraRecordFromLocalState());
}

function canSave(): boolean {
	return !needTest() && (isNewCamera.value || !areCameraConnectionsEqual(original.value, newCameraRecordFromLocalState()));
}

function saveButtonTitle(): string {
	if (busySaving.value) {
		return "Saving...";
	} else if (isNewCamera.value) {
		return "Add Camera";
	} else {
		return "Save Changes";
	}
}

function copyCameraRecordToLocalState(camera: CameraRecord) {
	host.value = camera.host;
	username.value = camera.username;
	password.value = camera.password;
	model.value = camera.model;
	name.value = camera.name;
}

function copyLocalStateToCameraRecord(rec: CameraRecord) {
	rec.host = host.value;
	rec.username = username.value;
	rec.password = password.value;
	rec.model = model.value;
	rec.name = name.value;
}

function newCameraRecordFromLocalState(): CameraRecord {
	let c = original.value.clone();
	copyLocalStateToCameraRecord(c);
	return c;
}

async function onSave(allowNavigate: boolean) {
	if (isNewCamera.value) {
		// Add the camera to the system
		busySaving.value = true;
		let r = await fetchOrErr('/api/config/addCamera', { method: "POST", body: JSON.stringify(newCameraRecordFromLocalState().toJSON()) });
		if (r.ok) {
			await globals.loadCameras();
		}
		busySaving.value = false;
		if (!r.ok) {
			error.value = r.error;
			return;
		}
		globals.lastCameraUsername = username.value;
		globals.lastCameraPassword = password.value;
		if (allowNavigate) {
			pushRoute(router, { name: returnRoute });
		}
	} else {
		busySaving.value = true;
		let r = await fetchOrErr('/api/config/changeCamera', { method: "POST", body: JSON.stringify(newCameraRecordFromLocalState().toJSON()) });
		if (r.ok) {
			await globals.loadCameras();
		}
		busySaving.value = false;
		if (!r.ok) {
			error.value = r.error;
			return;
		}
	}
}

function onTest() {
	testBusy.value = true;
}

function onTestFinished(result: CameraTestResult) {
	testBusy.value = false;
	if (result.error) {
		testResultImageBlob.value = null;
		error.value = result.error;
	} else if (result.image) {
		lastGoodConfig.value = newCameraRecordFromLocalState();
		testResultImageBlob.value = result.image;
		error.value = '';
		testResult.value = TestResult.Success;
	}
}

function onUnpair() {
	showConfirmUnpair.value = true;
}

async function onUnpairConfirmed() {
	showConfirmUnpair.value = false;
	unpairBusy.value = true;
	let r = await fetchOrErr(`/api/config/removeCamera/${props.id}`, { method: "POST" });
	unpairBusy.value = false;
	if (!r.ok) {
		error.value = r.error;
		return;
	}
	if (r.ok) {
		await globals.loadCameras();
	}
	pushRoute(router, { name: "rtSettingsHome" });
}

function unpairTitle() {
	if (unpairBusy.value) {
		return "Unpair Busy...";
	}
	return "Unpair Camera";
}

onMounted(async () => {
	if (!isNewCamera.value) {
		let r = await fetchOrErr(`/api/config/camera/${props.id}`);
		if (r.ok) {
			original.value = CameraRecord.fromJSON(await r.r.json());
			console.log(`Camera loaded. original=${original.value.id}, isNewCamera=${isNewCamera.value}`);
			copyCameraRecordToLocalState(original.value);
			lastGoodConfig.value = original.value.clone();
		}
	} else {
		let discoveredHost = window.location.hash.match(/host=([^&]+)/);
		let discoveredModel = window.location.hash.match(/model=([^&]+)/);
		let returnToScan = window.location.hash.match(/returnToScan=([^&]+)/);

		//console.log("EditCamera host", discoveredHost);
		//console.log("EditCamera model", discoveredModel);
		//console.log("globals.lastCameraPassword", globals.lastCameraPassword);
		//console.log("globals.lastCameraUsername", globals.lastCameraUsername);
		if (discoveredHost) {
			host.value = discoveredHost[1];
		}
		if (discoveredModel) {
			model.value = discoveredModel[1];
		}
		if (returnToScan) {
			returnRoute = 'rtSettingsScanForCameras';
		}
		if (globals.lastCameraPassword) {
			password.value = globals.lastCameraPassword;
		}
		if (globals.lastCameraUsername) {
			username.value = globals.lastCameraUsername;
		}

		// Pick a camera name
		let i = 1;
		let newName = '';
		while (true) {
			newName = `Camera ${i}`;
			if (!globals.cameras.find(c => c.name === newName)) {
				break;
			}
			i++;
		}
		name.value = newName;
	}
	// We need to wait for the right/left swipe transition to finish, before we can scroll
	// SYNC-SLIDE-TRANSITION
	//await sleep(185);
	//scrollSaver.load();
})

onBeforeUnmount(() => {
	//scrollSaver.save();
})

</script>

<template>
	<wide-root :title="isNewCamera ? 'Add Camera' : 'Edit Camera'">
		<wide-section>
			<wide-input label="Camera Name" v-model="name" @change="onNameChanged" :required="true" />
		</wide-section>
		<wide-section>
			<wide-input label="IP Address / Hostname" v-model="host" okText="OK" :required="true" />
			<wide-input label="Model" v-model="model" :options="constants.cameraModels" :required="true" />
			<wide-input label="Username" v-model="username" okText="OK" :required="true" />
			<wide-input label="Password" v-model="password" type="password" autocomplete="off" okText="OK"
				:required="true" />
			<div v-if="error" class="error">{{ error }}</div>
			<div class="submit">
				<button :class="{ focalButton: !canSave(), submitButtons: true }" :disabled="isTestDisabled()"
					@click="onTest">Test
					Settings</button>
				<div style="width:10px" />
				<button :class="{ focalButton: canSave(), submitButtons: true }" :disabled="!canSave() || busySaving"
					@click="onSave(true)">{{
						saveButtonTitle()
					}}</button>
			</div>
			<div v-if="!isNewCamera || testResultImageBlob || testBusy" class="previewContainer">
				<camera-preview :camera="original" :image-blob="testResultImageBlob" />
				<camera-tester v-if="testBusy" :camera="newCameraRecordFromLocalState()" @close="onTestFinished" />
			</div>
		</wide-section>
		<wide-section v-if="!isNewCamera">
			<wide-button :routeTarget="`/settings/camera/${id}/detectionZone`">Detection Zone</wide-button>
		</wide-section>
		<wide-section v-if="!isNewCamera">
			<wide-button class="unpair" @click="onUnpair" :disabled="unpairBusy">{{ unpairTitle() }}</wide-button>
		</wide-section>
		<confirm v-if="showConfirmUnpair" msg="Are you sure you want to unlink this camera?" yesText="Remove Camera"
			:danger='true' @cancel="showConfirmUnpair = false" @ok="onUnpairConfirmed" />
	</wide-root>
</template>

<style lang="scss" scoped>
@import '@/assets/vars.scss';
@import '@/components/widewidgets/widewidget.scss';

.spacer {
	height: 10px;
}

// I can't get rid of the border around this image!!! never seen this before....
// hmm now it's gone.
.preview {
	margin: 10px 0px 10px 0px;
	width: 200px;
	min-height: 100px;
	border-radius: 3px;
}

.submit {
	display: flex;
	flex-direction: row;
	justify-content: center;
	margin: 16px 16px 24px 16px;
}

.submitButtons {
	padding: 0.4em 0.7em;
	font-size: 16px;
	//font-weight: 500;
}

.success {
	color: #0a0;
	margin: 16px 20px 0px 20px;
	display: flex;
	justify-content: center;
}

.error {
	color: #d00;
	padding: 10px 0 0 0;
	margin: 16px 20px 0px 20px;
	display: flex;
	justify-content: center;
}

.previewContainer {
	display: flex;
	justify-content: center;
	margin-bottom: 16px;
}

.unpair {
	color: #d00;
	padding: 16px 12px;
	//font-weight: 500;
}
</style>
