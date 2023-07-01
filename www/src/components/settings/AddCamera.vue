<script setup lang="ts">
import { CameraRecord } from '@/db/config/configdb';
import { onMounted, ref } from 'vue';
import NewCameraConfig from '@/components/settings/NewCameraConfig.vue';
import { encodeQuery, fetchOrErr } from '@/util/util';
import Error from '@/components/core/Error.vue';
import Buttin from '@/components/core/Buttin.vue';
import WideButton from '@/components/widewidgets/WideButton.vue';
import ScannedCamera from './ScannedCamera.vue';
import { useRouter } from 'vue-router';
import { pushRoute } from "@/router/helpers";
import TopologyStar3 from '@/icons/topology-star-3.svg';
import TextPlus from '@/icons/text-plus.svg';

const router = useRouter();

let props = defineProps({
	isInitialSetup: {
		type: Boolean,
		default: false,
	}
});

let emits = defineEmits(['finished']);

enum Modes {
	ScanOrManual,
	Scanning,
	ScanResults,
	Details,
}

let mode = ref(Modes.ScanOrManual);
let error = ref('');
let configured = ref([] as CameraRecord[]); // cameras in the server DB
let scanned = ref([] as CameraRecord[]); // scanned on the LAN, and NOT in the DB
let busyScanning = ref(false);
let numTotalScans = ref(0);
let numExplicitScans = 0;
let expanded = ref(null as CameraRecord | null);

onMounted(async () => {
	fetchExisting();
	//if (props.isInitialSetup) {
	//	scan(false);
	//}
})

async function onScanAgain() {
	scan(true);
}

async function scan(isExplicit: boolean) {
	numTotalScans.value++;
	if (isExplicit) {
		numExplicitScans++;
	}
	// raise the timeout with each scan, in case there are slow cameras
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
}

function isConfigured(cam: CameraRecord): boolean {
	for (let c of configured.value) {
		if (c.host === cam.host) {
			return true;
		}
	}
	return false;
}

function onManual() {
	pushRoute(router, { name: 'rtSettingsEditCamera', params: { id: 'new' } });
}

</script>

<template>
	<div class="wideRoot">
		<div v-if="mode === Modes.ScanOrManual">
			<wide-button :icon="TopologyStar3">Scan local network for cameras</wide-button>
			<wide-button :icon="TextPlus" @click="onManual">Enter camera details manually</wide-button>
		</div>
		<div v-else-if="mode === Modes.Details">
		</div>
		<!--
		<div>Add Camera z</div>
		<error v-if="error">{{ error }}</error>
		<div v-if="numTotalScans !== 0" class="scanned">
			<p style="margin-bottom: 30px">The following devices were found on your network</p>
			<div v-for="camera of scanned">
				<div v-if="expanded === camera" class="expanded shadow9">
					<new-camera-config :camera="camera" @add="onAddCamera(camera)" />
				</div>
				<scanned-camera v-if="expanded !== camera" :camera="camera" :is-configured="isConfigured(camera)"
					@add="expanded = camera" style="margin: 10px 0px" />
			</div>
			<div class="flex" style="justify-content: flex-end; margin: 25px 0px;">
				<buttin :busy="busyScanning" @click="onScanAgain">Scan Again</buttin>
			</div>
		</div>
		<div v-else>
			<div class="flex" style="justify-content: flex-end; margin: 25px 0px;">
				<buttin :busy="busyScanning" @click="onScanAgain">Scan Local Network</buttin>
			</div>
		</div>
		<buttin>I know the camera details</buttin>
		-->
	</div>
</template>

<style lang="scss" scoped>
.expanded {
	padding: 15px 10px 10px 20px;
	border-radius: 5px;
}
</style>
