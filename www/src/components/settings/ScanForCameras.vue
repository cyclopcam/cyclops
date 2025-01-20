<script setup lang="ts">
import { CameraRecord } from '@/db/config/configdb';
import { onMounted, ref } from 'vue';
import { encodeQuery, fetchOrErr, sleep } from '@/util/util';
import Buttin from '@/components/core/Buttin.vue';
import WideRoot from '@/components/widewidgets/WideRoot.vue';
import WideSection from '@/components/widewidgets/WideSection.vue';
import ScannedCamera from './ScannedCamera.vue';
import { useRouter } from 'vue-router';
import { pushRoute } from "@/router/helpers";
import { globals } from '@/globals';

const router = useRouter();

let props = defineProps<{
}>();

let emits = defineEmits(['finished']);

let error = ref('');
let configured = ref([] as CameraRecord[]); // cameras in the server DB
let scanned = ref([] as CameraRecord[]); // scanned on the LAN, could be configured or not
let busyScanning = ref(false);
let timeoutMS = ref(0);
let numScans = ref(0);

// Use this to simulate a scan, when iterating on the UI/UX
let enableDebugFakeScan = false;
async function debugFakeScan() {
	await sleep(500);
	scanned.value = [];
}

async function onScanAgain() {
	scan();
}

async function scan() {
	numScans.value++;
	// Raise the timeout with each scan, in case there are slow cameras.
	// Initially on my home network, 150ms was enough to find my 3 cameras. But then something happened,
	// and I needed to raise that limit by 10x, to 1500ms, in order to be reliable. I don't know yet
	// what caused this.
	timeoutMS.value = 1500 * Math.pow(2, Math.min(numScans.value - 1, 3));
	let options = {
		"timeout": timeoutMS.value,
		"includeExisting": "1", // I think it just looks better if we include existing cameras in our scan - more consistency and user confidence
	};
	scanned.value = [];
	busyScanning.value = true;
	if (enableDebugFakeScan) {
		await debugFakeScan();
	} else {
		let r = await fetchOrErr('/api/config/scanNetworkForCameras?' + encodeQuery(options), { method: 'POST' });
		if (r.ok) {
			let resultJSON = await r.r.json();
			globals.lastNetworkCameraScanJSON = resultJSON;
			scanned.value = (resultJSON as []).map(x => CameraRecord.fromJSON(x));
		} else {
			error.value = r.error;
		}
	}
	busyScanning.value = false;
}

async function fetchExisting() {
	let r = await fetchOrErr('/api/config/cameras');
	if (r.ok) {
		configured.value = ((await r.r.json()) as []).map(x => CameraRecord.fromJSON(x));
	}
}

function onAdd(cam: CameraRecord) {
	// Only return to this page if this is not the final camera in the scanned list
	let remaining = 0;
	for (let c of scanned.value) {
		if (!isConfigured(c)) {
			remaining++;
		}
	}
	let returnToScan = remaining > 1 ? '1' : '0';

	pushRoute(router, { name: 'rtSettingsEditCamera', params: { id: 'new', host: cam.host, model: cam.model, returnToScan: returnToScan } });
}

function isConfigured(cam: CameraRecord): boolean {
	for (let c of configured.value) {
		if (c.host === cam.host) {
			return true;
		}
	}
	return false;
}

onMounted(async () => {
	// Start the 'in progress' animation immediately, so the user knows something is happening.
	busyScanning.value = true;

	await fetchExisting();

	console.log(`ScanForCameras onMounted, lastNetworkCameraScanJSON=`, globals.lastNetworkCameraScanJSON);
	if (globals.lastNetworkCameraScanJSON) {
		// The user is busy setting up a bunch of cameras, so we don't want to rescan after each one.
		// The user can always just hit Scan again, if this list is stale.
		scanned.value = (globals.lastNetworkCameraScanJSON as []).map(x => CameraRecord.fromJSON(x));
	} else {
		await scan();
	}

	busyScanning.value = false;
})

</script>

<template>
	<wide-root title="Scan for Cameras">
		<wide-section>
			<div class="title">The following devices were found on your network</div>
			<div v-if="busyScanning" class="busy">
				Busy Scanning (timeout {{ timeoutMS / 1000 }} seconds)...
			</div>
			<div v-else-if="!busyScanning && scanned.length === 0">
				<span style="color:#d00">No cameras found.</span><br><br>If your server is running in a different subnet
				to
				your cameras, you can
				specify the subnet to scan with the "--ip &lt;network&gt;" CLI parameter when starting the server.
				<br><br>
				Each time you press "Scan Again", we'll raise the network timeout.
			</div>
			<div class="cameras">
				<div v-for="camera of scanned">
					<scanned-camera :camera="camera" :is-configured="isConfigured(camera)" @add="onAdd(camera)"
						style="margin: 10px 0px" />
				</div>
				<div class="scanAgain">
					<buttin :busy="busyScanning" @click="onScanAgain">Scan Again</buttin>
				</div>
			</div>
		</wide-section>
	</wide-root>
</template>

<style lang="scss" scoped>
.title {
	padding: 30px 20px;
	text-align: center;
	font-size: 18px;
	font-weight: 500;
}

.busy {
	text-align: center;
}

.cameras {
	padding: 0px 8px;
}

.scanAgain {
	display: flex;
	justify-content: flex-end;
	margin: 30px 0px 20px 0px;
}
</style>
