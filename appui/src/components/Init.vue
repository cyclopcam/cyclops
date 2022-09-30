<script setup lang="ts">
import { globals } from '@/natcom';
import { reactive, ref } from 'vue';

// SYNC-SCAN-STATE	
interface ScanState {
	message: string;
	status: "i" | "b" | "e" | "s"; // i:initial, b:busy, e:error, s:success
	servers: string[];
	nScanned: number;
}

let showScanStatus = ref(false);
let scanState = reactive({ message: "", status: "i", servers: [], nScanned: 0 } as ScanState);

function onScan() {
	showScanStatus.value = true;

	// debug UI
	//scanState.status = 'b';
	//scanState.nScanned = 34;
	//scanState.servers = ["192.168.10.11"];

	fetch('/natcom/scanForServers', { method: 'POST' });
	pollStatus();
}

async function pollStatus() {
	let r = await (await fetch('/natcom/scanStatus')).json();
	scanState.message = r.message;
	scanState.status = r.status;
	scanState.servers = r.servers;
	scanState.nScanned = r.nScanned;
	if (scanState.status !== 's' && scanState.status !== 'e') {
		setTimeout(pollStatus, 300);
	}
}

</script>
 
<template>
	<div class="flexColumnCenter init">
		<h1 style="margin-bottom: 50px">Connect to your<br /> Cyclops system</h1>
		<button @click="onScan" :disabled="scanState.status === 'b'">Scan Home Network</button>
		<div v-if="showScanStatus" :class="{scanning: true}">
			<div :class="{error: scanState.status === 'e'}">{{scanState.message}}</div>
			<div style="'margin-top: 8px'">Scanned {{scanState.nScanned}} / 253</div>
			<div v-if="scanState.servers.length !== 0" style="margin-top: 10px">
				<div style="margin-bottom: 5px">Cyclops Servers</div>
				<div v-for="s of scanState.servers" :key="s" style="margin-left:10px">
					{{s}}
				</div>
			</div>
		</div>
	</div>
</template>

<style scoped lang="scss">
.init {
	margin: 10px;
}

.scanning {
	margin: 20px;
}

.error {
	color: #d00;
}
</style>
