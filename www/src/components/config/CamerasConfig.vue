<script setup lang="ts">
import { CameraRecord } from '@/db/config/configdb';
import { onMounted, reactive, ref } from 'vue';
import CameraConfig from '@/components/config/CameraConfig.vue';
import { encodeQuery, fetchOrErr } from '@/util/util';
import Error from '../core/Error.vue';
import Buttin from '../core/Buttin.vue';
import CameraConfigSummary from './CameraConfigSummary.vue';

let error = ref('');
let scanned = ref([] as CameraRecord[]);
let busyScanning = ref(false);
let numExplicitScans = 0;
//let isExpanded = reactive({} as { [index: string]: boolean });
let expanded = ref(null as CameraRecord | null);

onMounted(async () => {
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

</script>

<template>
	<div>
		<error v-if="error">{{ error }}</error>
		<div class="scanned">
			<p style="margin-bottom: 30px">The following devices were found on your network</p>
			<div v-for="camera of scanned" style="margin: 40px 0px">
				<div v-if="expanded === camera" class="expanded">
					<camera-config :camera="camera" />
				</div>
				<camera-config-summary v-if="expanded !== camera" :camera="camera" @add="expanded = camera" />
			</div>
			<div class="flex" style="justify-content: flex-end; margin: 30px 0px;">
				<buttin :busy="busyScanning" @click="onScanAgain">Scan Again</buttin>
			</div>
		</div>
	</div>
</template>

<style lang="scss" scoped>
.expanded {
	padding: 15px 20px;
	box-shadow: 3px 3px 9px rgba(0, 0, 0, 0.3);
	border-radius: 10px;
}
</style>
