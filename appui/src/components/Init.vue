<script setup lang="ts">
import { globals, maxIPsToScan } from '@/global';
import type { ScanState } from '@/global';
import { router, pushRoute } from '@/router/routes';
import { onMounted, reactive, ref } from 'vue';

function showScanStatus(): boolean {
	return globals.scanState.status !== "i";
};

function scanState(): ScanState {
	return globals.scanState;
}

function onScan() {
	let ss = scanState();
	fetch('/natcom/scanForServers', { method: 'POST' });
	pollStatus();
}

async function pollStatus() {
	let r = await (await fetch('/natcom/scanStatus')).json();
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

interface ParsedServer {
	ip: string;
	host: string;
}

function parsedServers(): ParsedServer[] {
	return scanState().servers.map(x => parseHostname(x));
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

function onClickLocal(s: ParsedServer) {
	pushRoute({ name: "rtConnectLocal" });
}

</script>
 
<template>
	<div class="flexColumnCenter init">
		<h1 style="margin-bottom: 40px">Connect to your<br /> Cyclops system</h1>
		<button @click="onScan" :disabled="scanState().status === 'b'">Scan Home Network</button>
		<div class="link" @click="onConnectExisting" style="margin-top: 20px">Connect to
			Existing Server</div>
		<div v-if="showScanStatus()" class="scanning shadow15L">
			<div :class="{block: true, error: scanState().error !== 'e'}">{{scanState().error}}</div>
			<div v-if="scanState().nScanned !== 0" class="block textCenter">
				Scanned {{scanState().nScanned}} / {{maxIPsToScan}}
				<div :style="progStyle()" />
			</div>
			<div v-if="scanState().servers.length !== 0 || (scanState().status === 'd' && scanState().error === '')"
				class="block" style="margin-top: 30px">
				<h3 style="margin-bottom: 15px;">Found {{scanState().servers.length}} Cyclops Servers
				</h3>
				<div v-for="s of parsedServers()" :key="s.ip" class="link server" @click="onClickLocal(s)">
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
	margin: 12px 0px;
}
</style>
