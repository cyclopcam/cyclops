<script setup lang="ts">
import { globals, ServerPort } from '@/global';
import { maxIPsToScan, parseServers } from '@/scan';
import { startScan, getScanStatus } from '@/nattypes';
import type { ScanState, ParsedServer } from '@/scan';
import { router, pushRoute } from '@/router/routes';
import { onMounted, reactive, ref } from 'vue';
import { encodeQuery } from '@/util/util';

let props = defineProps<{
	init: string,
	scanOnLoad: string,
}>()

function parsedServers(): ParsedServer[] {
	return parseServers(scanState());
}

function showScanStatus(): boolean {
	return globals.scanState.status !== "i";
};

function scanState(): ScanState {
	return globals.scanState;
}

function onScan() {
	let ss = scanState();
	startScan();
	pollStatus();
}

async function pollStatus() {
	let r = await getScanStatus();
	let ss = scanState();
	ss.error = r.error;
	ss.phoneIP = r.phoneIP;
	ss.status = r.status;
	ss.servers = r.servers;
	ss.nScanned = r.nScanned;
	if (ss.status === 'b') {
		setTimeout(pollStatus, 300);
	}
}

function progStyle() {
	let finished = scanState().nScanned === maxIPsToScan;
	return {
		"width": (scanState().nScanned * 100 / maxIPsToScan) + "%",
		"height": "4px",
		"margin-top": "4px",
		"background-color": finished ? "#333" : "#aaa",
	}
}

function onConnectExisting() {
	pushRoute({ name: "rtConnectExisting" });
}

async function onClickLocal(s: ParsedServer) {
	let baseUrl = `http://${s.ip}:${ServerPort}`;
	await fetch('/natcom/navigateToScannedLocalServer?' + encodeQuery({ url: baseUrl }));
	//pushRoute({ name: "rtConnectLocal", params: { ip: s.ip, host: s.host } });
}

onMounted(() => {
	if (props.scanOnLoad === "1") {
		onScan();
	}
})

</script>
 
<template>
	<div class="flexColumnCenter pageContainer init">
		<h1 style="margin-bottom: 40px">Connect to your<br /> Cyclops system</h1>
		<button @click="onScan" :disabled="scanState().status === 'b'">Scan Home Network</button>
		<div v-if="init === '1'" class="link" @click="onConnectExisting" style="margin-top: 20px">Connect to
			Remote Server</div>
		<div v-if="showScanStatus()" class="scanning shadow15L">
			<div :class="{ block: true, error: scanState().error !== 'e' }">{{ scanState().error }}
			</div>
			<div v-if="scanState().nScanned !== 0" class="block textCenter">
				Scanned {{ scanState().nScanned }} / {{ maxIPsToScan }}
				<div :style="progStyle()" />
			</div>
			<div v-if="scanState().servers.length !== 0 || (scanState().status === 'd' && scanState().error === '')"
				class="block" style="margin-top: 30px">
				<h3 style="margin-bottom: 15px;">Found {{ scanState().servers.length }} Cyclops Servers
				</h3>
				<div v-for="s of parsedServers()" :key="s.ip" :class="{ link: true, server: true }"
					@click="onClickLocal(s)">
					{{ s.ip }}
					<span style="margin-left: 5px">{{ s.host }}</span>
				</div>
			</div>
		</div>
	</div>
</template>

<style scoped lang="scss">
.init {}

.block {
	margin: 10px 0px;
}

.scanning {
	margin: 30px 10px;
	padding: 5px 20px;
	border-radius: 10px;
}

.textCenter {
	text-align: center;
}

.error {
	color: #d00;
}

.server {
	margin: 20px 0px;
}
</style>
