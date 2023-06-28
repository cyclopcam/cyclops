<script setup lang="ts">
import type { CameraRecord } from '@/db/config/configdb';
import { onMounted, ref } from 'vue';
import { Context } from '../form/forms';
import FormText from '../form/FormText.vue';
import FormDropdown from '../form/FormDropdown.vue';
import FormBottom from '../form/FormBottom.vue';
import { constants } from '@/constants';
import CameraTester from './CameraTester.vue';
import { addToRecentPasswords, addToRecentUsernames, recentPasswords, recentUsernames, type CameraTestResult } from './config';
import { fetchOrErr } from '@/util/util';

// THIS FILE IS DEPRECATED

let props = defineProps<{
	camera: CameraRecord
}>();
let emits = defineEmits(['add']);

let preview = ref(null);

let ctx = new Context(() => {
	if (host.value === '' || model.value === '') {
		return false;
	}
	if (testGood.value) {
		if (name.value === '') {
			return false;
		}
	}
	return true;
}, { inputWidth: '170px', name: 'CameraConfig' });
let testBusy = ref(false);
let havePreview = ref(false);
let testGood = ref(false);

let host = ref(props.camera.host);
let username = ref(props.camera.username);
let password = ref(props.camera.password);
let model = ref(props.camera.model);
let name = ref(props.camera.name);

function ownConfig(): CameraRecord {
	let c = props.camera.clone();
	c.host = host.value;
	c.username = username.value;
	c.password = password.value;
	c.model = model.value;
	c.name = name.value;
	return c;
}

async function onTestOrAdd() {
	if (testGood.value) {
		// we're all good - add the camera to the system
		let r = await fetchOrErr('/api/config/addCamera', { method: "POST", body: JSON.stringify(ownConfig().toJSON()) });
		if (!r.ok) {
			ctx.submitError.value = r.error;
			return;
		}
		emits('add', ownConfig());
	} else {
		testBusy.value = true;
	}
}

function onTestClose(result: CameraTestResult) {
	testBusy.value = false;
	//console.log("onTestClose", result);
	if (result.error) {
		ctx.submitError.value = result.error;
	} else if (result.image) {
		let url = window.URL.createObjectURL(result.image);
		if (preview.value) {
			//console.log("Setting preview image");
			(preview.value as HTMLImageElement).src = url;
		}
		ctx.submitError.value = '';
		havePreview.value = true;
		testGood.value = true;
		addToRecentUsernames(username.value);
		addToRecentPasswords(password.value);
	}
}

onMounted(() => {
	if (username.value === '' && recentUsernames.length !== 0) {
		username.value = recentUsernames[recentUsernames.length - 1];
	}
	if (password.value === '' && recentPasswords.length !== 0) {
		password.value = recentPasswords[recentPasswords.length - 1];
	}
})

</script>

<template>
	<div class="flexColumn">
		<div class="flex">
			<div class="flexColumn">
				<form-text :ctx="ctx" label="IP Address / Hostname" v-model="host" placeholder="ip/hostname"
					:required="true" />
				<div class="spacer" />
				<form-dropdown :ctx="ctx" label="Model" v-model="model" placeholder="model" :required="true"
					:options="constants.cameraModels" />
				<div class="spacer" />
				<form-text :ctx="ctx" label="Username" v-model="username" placeholder="username" autocomplete="username" />
				<div class="spacer" />
				<form-text :ctx="ctx" label="Password" v-model="password" placeholder="password" :password="true" />
				<div class="spacer" />
				<form-text :ctx="ctx" label="Camera Name" v-model="name" placeholder="camera name" :required="testGood" />
				<div class="spacer" />
			</div>
			<div style="width: 30px" />
			<div class="flexColumnCenter">
				<div style="height: 15px" />
				<img ref="preview" class="preview shadow5L" :style="{ visibility: havePreview ? 'visible' : 'hidden' }" />
				<div v-if="testGood" style="margin: 10px 5px; color: #080">Success!</div>
			</div>
		</div>
		<form-bottom :ctx="ctx" :submit-title="testGood ? 'Add Camera' : 'Test Connection'" @submit="onTestOrAdd" />
		<camera-tester v-if="testBusy" :camera="ownConfig()" @close="onTestClose" />
	</div>
</template>

<style lang="scss" scoped>
.spacer {
	height: 10px;
}

// I can't get rid of the border around this image!!! never seen this before....
.preview {
	width: 140px;
	min-height: 80px;
	border-radius: 3px;
}
</style>
