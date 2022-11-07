<script setup lang="ts">
import { globals, ServerPort } from '@/global';
import { maxIPsToScan } from '@/scan';
import { natStartScan, natGetScanStatus, natNavigateToScannedLocalServer, natSetLocalWebviewVisibility, LocalWebviewVisibility, registeredFakeServers, fakeServerList } from '@/nativeOut';
import type { ScanState, ScannedServer } from '@/scan';
import { router, pushRoute } from '@/router/routes';
import { onMounted, reactive, ref } from 'vue';
import { encodeQuery, sleep } from '@/util/util';
import { showToast } from './widgets/toast';
import { dummyMode } from '@/constants';

//let props = defineProps<{}>()

function isInit(): boolean {
	return globals.mustShowWelcomeScreen;
}

function showScanStatus(): boolean {
	return globals.scanState.status !== "i";
};

function scanState(): ScanState {
	return globals.scanState;
}

async function onScan() {
	await natStartScan();
	pollStatus();
}

async function pollStatus() {
	let r = await natGetScanStatus();
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
		"background-color": finished ? "#080" : "#aaa",
	}
}

function onConnectExisting() {
	pushRoute({ name: "rtConnectExisting" });
}

async function onClickLocal(ev: MouseEvent, s: ScannedServer) {
	let existing = globals.servers.find(x => x.publicKey === s.publicKey);
	if (existing) {
		showToast("You are already connected to this server");
		return;
	}
	if (dummyMode) {
		// simulate as best we can, what happens on the native mobile side
		showToast("Assuming successful login");
		await sleep(500);
		registeredFakeServers.push(fakeServerList.find(x => x.publicKey === s.publicKey)!);
		globals.mustShowWelcomeScreen = false;
		globals.showExpanded(false);
		(window as any).cyRefreshServers();
	} else {
		await natSetLocalWebviewVisibility(LocalWebviewVisibility.Hidden);
		await natNavigateToScannedLocalServer(s);
	}
}

onMounted(() => {
	// On our welcome screen, we want the user to have time to read what's going on,
	// and not just scan the LAN immediately.
	if (!isInit()) {
		onScan();
	}
})

</script>
 
<template>
	<div :class="{ flexColumnCenter: true, pageContainer: true, init: isInit() }">
		<h1 style="margin-bottom: 45px">Connect to your<br /> Cyclops system</h1>
		<button class="focalButton" @click="onScan" :disabled="scanState().status === 'b'">Scan Home Network</button>
		<div v-if="isInit()" class="link" @click="onConnectExisting" style="margin-top: 30px; font-size: 13px">Connect
			to
			Remote Server</div>
		<div v-if="showScanStatus()" class="scanning shadow15L">
			<div :class="{ block: true, error: scanState().error !== 'e' }">{{ scanState().error }}
			</div>
			<div v-if="scanState().nScanned !== 0"
				:class="{ block: true, textCenter: true, scanFinished: scanState().status === 'd' }">
				Scanned {{ scanState().nScanned }} / {{ maxIPsToScan }}
				<div :style="progStyle()" />
			</div>
			<div v-if="scanState().servers.length !== 0 || (scanState().status === 'd' && scanState().error === '')"
				class="block" style="margin-top: 30px">
				<h3 style="margin-bottom: 15px;">Found {{ scanState().servers.length }} Cyclops Servers
				</h3>
				<div v-for="s of scanState().servers" :key="s.ip" :class="{ link: true, server: true }"
					@click="onClickLocal($event, s)">
					{{ s.ip }}
					<span style="margin-left: 5px">{{ s.hostname }}</span>
				</div>
			</div>
		</div>
	</div>
</template>

<style scoped lang="scss">
.init {
	height: 100%;
}

.block {
	margin: 10px 0px;
}

.scanning {
	margin: 30px 10px;
	padding: 5px 20px;
	border-radius: 10px;
}

.scanFinished {
	color: #080;
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
