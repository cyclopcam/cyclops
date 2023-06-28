<script setup lang="ts">
import { CameraRecord } from '@/db/config/configdb';
import { computed, onMounted, ref } from 'vue';
import { constants } from '@/constants';
import CameraTester from './CameraTester.vue';
import CameraPreview from './CameraPreview.vue';
import { addToRecentPasswords, addToRecentUsernames, recentPasswords, recentUsernames, type CameraTestResult } from './config';
import { fetchOrErr } from '@/util/util';
import WideText from '@/components/widewidgets/WideText.vue';
import WideDropdown from '@/components/widewidgets/WideDropdown.vue';
import WideButton from '@/components/widewidgets/WideButton.vue';
import WideSection from '@/components/widewidgets/WideSection.vue';
import WideSpacer from '@/components/widewidgets/WideSpacer.vue';
import Confirm from '@/components/widgets/Confirm.vue';
import { useRouter } from 'vue-router';
import { pushRoute } from "@/router/helpers";

let props = defineProps<{
	id: string, // either the ID or "new"
}>();

let router = useRouter();

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
let showConfirmUnpair = ref(false);
let error = ref('');

// This is useful for development on the Edit Camera workflow, because it allows you
// to change a trivial field and then go through the Test Camera.. Save Changes process.
let nameChangeNeedsTest = true;

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
	if (isNewCamera.value) {
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

async function onSave() {
	if (isNewCamera.value) {
		// Add the camera to the system
		let r = await fetchOrErr('/api/config/addCamera', { method: "POST", body: JSON.stringify(newCameraRecordFromLocalState().toJSON()) });
		if (!r.ok) {
			error.value = r.error;
			return;
		}
		pushRoute(router, { name: "rtSettingsHome" });
	} else {
		let r = await fetchOrErr('/api/config/changeCamera', { method: "POST", body: JSON.stringify(newCameraRecordFromLocalState().toJSON()) });
		if (!r.ok) {
			error.value = r.error;
			return;
		}
		pushRoute(router, { name: "rtSettingsHome" });
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
		addToRecentUsernames(username.value);
		addToRecentPasswords(password.value);
	}
}

function onUnpair() {
	showConfirmUnpair.value = true;
}

async function onUnpairConfirmed() {
	showConfirmUnpair.value = false;
	let r = await fetchOrErr('/api/config/addCamera', { method: "POST", body: JSON.stringify(newCameraRecordFromLocalState().toJSON()) });
	if (!r.ok) {
		error.value = r.error;
		return;
	}
}

onMounted(async () => {
	if (!isNewCamera.value) {
		let r = await fetchOrErr(`/api/config/camera/${props.id}`);
		if (r.ok) {
			original.value = CameraRecord.fromJSON(await r.r.json());
			//console.log(`Camera loaded. original=${original.value.id}, isNewCamera=${isNewCamera.value}`);
			copyCameraRecordToLocalState(original.value);
			lastGoodConfig.value = original.value.clone();
		}
		if (username.value === '' && recentUsernames.length !== 0) {
			username.value = recentUsernames[recentUsernames.length - 1];
		}
		if (password.value === '' && recentPasswords.length !== 0) {
			password.value = recentPasswords[recentPasswords.length - 1];
		}
	}
})

</script>

<template>
	<div class="editRoot">
		<wide-text label="Camera Name" v-model="name" />
		<wide-text label="IP Address / Hostname" v-model="host" />
		<wide-dropdown label="Model" v-model="model" :options="constants.cameraModels" />
		<wide-text label="Username" v-model="username" />
		<wide-text label="Password" v-model="password" :password="true" autocomplete="off" />
		<wide-section>
			<!-- <div v-if="testResult === TestResult.Success" class="success">Connection Succeeded</div> -->
			<div v-if="error" class="error">{{ error }}</div>
			<div class="submit">
				<button :class="{ focalButton: !canSave(), submitButtons: true }" :disabled="isTestDisabled()"
					@click="onTest">Test
					Settings</button>
				<div style="width:10px" />
				<button :class="{ focalButton: canSave(), submitButtons: true }" :disabled="!canSave()" @click="onSave">{{
					saveButtonTitle()
				}}</button>
			</div>
			<div class="previewContainer">
				<camera-preview :camera="original" :image-blob="testResultImageBlob" />
				<camera-tester v-if="testBusy" :camera="newCameraRecordFromLocalState()" @close="onTestFinished" />
			</div>
		</wide-section>
		<wide-spacer />
		<wide-button class="unpair" @click="onUnpair">Unpair Camera</wide-button>
		<wide-spacer />
		<confirm v-if="showConfirmUnpair" msg="Are you sure you want to delete this camera?" yesText="Remove Camera"
			:danger='true' @cancel="showConfirmUnpair = false" @ok="onUnpairConfirmed" />
	</div>
</template>

<style lang="scss" scoped>
.editRoot {
	display: flex;
	flex-direction: column;
	width: 100%;
	box-sizing: border-box;
	//padding: 8px 20px;
}

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
