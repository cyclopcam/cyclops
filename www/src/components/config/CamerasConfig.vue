<script setup lang="ts">
import { CameraRecord } from '@/db/config/configdb';
import { onMounted, ref } from 'vue';
import CameraConfig from '@/components/config/CameraConfig.vue';
import { encodeQuery, fetchOrErr } from '@/util/util';
import Error from '../core/Error.vue';
import Buttin from '../core/Buttin.vue';
import CameraConfigSummary from './CameraConfigSummary.vue';
import Separator from '../core/separator.vue';

let error = ref('');
let configured = ref([] as CameraRecord[]); // cameras in the server DB
let scanned = ref([] as CameraRecord[]); // scanned on the LAN, and NOT in the DB
let busyScanning = ref(false);
let numExplicitScans = 0;
let expanded = ref(null as CameraRecord | null);
let sureContinue = ref("");
let numCamerasAdded = 0;

onMounted(async () => {
	fetchExisting();
	scan(false);
})

async function onScanAgain() {
	scan(true);
}

async function scan(isExplicit: boolean) {
	if (isExplicit) {
		numExplicitScans++;
	}
	let timeoutMS = 50 * Math.pow(2, Math.min(numExplicitScans, 3));
	let options = {
		"cache": isExplicit ? "nocache" : "",
		"timeout": timeoutMS
	};
	scanned.value = [];
	busyScanning.value = true;
	let r = await fetchOrErr('/api/config/scanNetworkForCameras?' + encodeQuery(options), { method: 'POST' });
	if (r.ok) {
		scanned.value = ((await r.r.json()) as []).map(x => CameraRecord.fromJSON(x));
	} else {
		error.value = r.error;
	}
	busyScanning.value = false;
}

async function fetchExisting() {
	let r = await fetchOrErr('/api/config/cameras');
	if (r.ok) {
		configured.value = ((await r.r.json()) as []).map(x => CameraRecord.fromJSON(x));
	}
}

function onAddCamera(cam: CameraRecord) {
	configured.value.push(cam);
	expanded.value = null;
	numCamerasAdded++;
}

function onContinue() {
	if (sureContinue.value === '' && configured.value.length === 0) {
		sureContinue.value = "You haven't added any cameras yet. Are you sure you want to continue?"
	}
}

function isConfigured(cam: CameraRecord): boolean {
	for (let c of configured.value) {
		if (c.host === cam.host) {
			return true;
		}
	}
	return false;
}

// Return the list of scanned cameras, which have not been configured
//let newScanned = computed((): CameraRecord[] => {
//	let r = [];
//	for (let s of scanned.value) {
//		if (!isConfigured(s)) {
//			r.push(s);
//		}
//	}
//	return r;
//});

</script>

<template>
	<div>
		<error v-if="error">{{ error }}</error>
		<div class="scanned">
			<p style="margin-bottom: 30px">The following devices were found on your network</p>
			<div v-for="camera of scanned">
				<div v-if="expanded === camera" class="expanded shadow9">
					<camera-config :camera="camera" @add="onAddCamera(camera)" />
				</div>
				<camera-config-summary v-if="expanded !== camera" :camera="camera" :is-configured="isConfigured(camera)"
					@add="expanded = camera" style="margin: 10px 0px" />
			</div>
			<div class="flex" style="justify-content: flex-end; margin: 25px 0px;">
				<buttin :busy="busyScanning" @click="onScanAgain">Scan Again</buttin>
			</div>
		</div>
		<separator />
		<div class="flex" style="justify-content: flex-end; align-items: center">
			<div v-if="sureContinue" style="color: #d80; max-width: 260px; margin-right: 20px">{{ sureContinue }}</div>
			<button @click="onContinue">{{ sureContinue ? "Yes, Continue" : "Continue" }}</button>
		</div>
	</div>
</template>

<style lang="scss" scoped>
.expanded {
	padding: 15px 10px 10px 20px;
	border-radius: 5px;
}
</style>
