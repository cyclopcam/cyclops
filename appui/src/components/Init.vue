<script setup lang="ts">
import { globals } from '@/natcom';
import { onMounted, reactive, ref } from 'vue';

// SYNC-SCAN-STATE	
interface ScanState {
	error: string; // If not empty, then status will be "d", and scan has stopped
	phoneIP: string;
	//status: "i" | "b" | "e" | "s"; // i:initial, b:busy, e:error, s:success
	status: "i" | "b" | "d"; // i:initial, b:busy, d:done
	servers: string[];
	nScanned: number;
}

const maxIPsToScan = 253;

let showScanStatus = ref(false);
let scanState = reactive({ error: "", phoneIP: "", status: "i", servers: [], nScanned: 0 } as ScanState);

// Used for UI design without having to do an IP scan
function mockScanState() {
	showScanStatus.value = true;
	scanState.phoneIP = "192.168.10.65";
	scanState.error = "";
	scanState.nScanned = 253;
	scanState.servers = ["192.168.10.11 (cyclops)", "192.168.10.15 (mars)"];
	scanState.status = "d";
}

function mockScanStateError() {
	showScanStatus.value = true;
	scanState.phoneIP = "";
	scanState.error = "Android Internal Error, Java foo bar etc etc. Errors are often long. Failed to get WiFi IP address";
	scanState.nScanned = 0;
	scanState.servers = [];
	scanState.status = "d";
}

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
	scanState.error = r.error;
	scanState.phoneIP = r.phoneIP;
	scanState.status = r.status;
	scanState.servers = r.servers;
	scanState.nScanned = r.nScanned;
	if (scanState.status === 'b') {
		setTimeout(pollStatus, 300);
	}
}

interface ParsedServer {
	ip: string;
	host: string;
}

function parsedServers(): ParsedServer[] {
	return scanState.servers.map(x => parseHostname(x));
}

// Split a string like "192.168.10.11 (rpi)" into the IP and hostname portions. Parentheses are removed.
function parseHostname(hostname: string): ParsedServer {
	let space = hostname.indexOf(' ');
	if (space === -1) {
		return { ip: hostname, host: '' };
	}
	return { ip: hostname.substring(0, space), host: hostname.substring(space + 2, hostname.length - 1) };
}

function progStyle() {
	let finished = scanState.nScanned === maxIPsToScan;
	return {
		"width": (scanState.nScanned * 100 / maxIPsToScan) + "%",
		"height": "4px",
		"margin-top": "4px",
		"background-color": finished ? "#333" : "#aaa",
	}
}

onMounted(() => mockScanState());
//onMounted(() => mockScanStateError());

</script>
 
<template>
	<div class="flexColumnCenter init">
		<h1 style="margin-bottom: 40px">Connect to your<br /> Cyclops system</h1>
		<button @click="onScan" :disabled="scanState.status === 'b'">Scan Home Network</button>
		<div class="link" @click="onScan" style="margin-top: 20px">Connect to
			Existing Server</div>
		<div v-if="showScanStatus" :class="{scanning: true}">
			<div :class="{block: true, error: scanState.error !== 'e'}">{{scanState.error}}</div>
			<div v-if="scanState.nScanned !== 0" class="block textCenter">
				Scanned {{scanState.nScanned}} / {{maxIPsToScan}}
				<div :style="progStyle()" />
			</div>
			<div v-if="scanState.servers.length !== 0 || (scanState.status === 'd' && scanState.error === '')"
				class="block" style="margin-top: 30px">
				<h3 style="margin-bottom: 15px;">Found {{scanState.servers.length}} Cyclops Servers
				</h3>
				<div v-for="s of parsedServers()" :key="s.ip" class="link server">
					{{s.ip}}
					<span style="margin-left: 5px">{{s.host}}</span>
				</div>
			</div>
		</div>
	</div>
</template>

<style scoped lang="scss">
.init {
	margin: 10px;
}

.block {
	margin-top: 10px;
}

.scanning {
	margin: 20px 15px;
}

.textCenter {
	text-align: center;
}

.error {
	color: #d00;
	margin-bottom: 30px;
}

.server {
	margin: 12px 0px;
}
</style>
