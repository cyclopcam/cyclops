<script setup lang="ts">
import MobileFullscreen from '@/components/responsive/MobileFullscreen.vue';
import NewUser from '@/components/settings/NewUser.vue';
import SetupVPN from '@/components/settings/SetupVPN.vue';
import { onMounted, ref } from 'vue';
import { computed } from '@vue/reactivity';
import SetupCameras from '../components/settings/SetupCameras.vue';
import { globals } from '@/globals';
import { replaceRoute, router } from "@/router/routes";
import SystemVariables from "../components/settings/SystemVariables.vue";
import { fetchWithAuth } from '@/util/util';

enum Stages {
	CreateFirstUser = 0,
	SetupVPN = 1,
	ConfigureVariables = 2,
	ConfigureCameras = 3,
}

let stage = ref(Stages.CreateFirstUser);
let isCreateFirstUser = computed(() => stage.value === Stages.CreateFirstUser);
let isSetupVPN = computed(() => stage.value === Stages.SetupVPN);
let isConfigureVariables = computed(() => stage.value === Stages.ConfigureVariables);
let isConfigureCameras = computed(() => stage.value === Stages.ConfigureCameras);

function stageText() {
	switch (stage.value) {
		case Stages.CreateFirstUser:
			return "Create a username and password for yourself";
		case Stages.SetupVPN:
			return "VPN Activation";
		case Stages.ConfigureVariables:
			return "System configuration";
		case Stages.ConfigureCameras:
			return "Let's find your cameras";
	}
}

async function moveToNextStage() {
	//console.log("moveToNextStage");
	if (stage.value === Stages.ConfigureCameras) {
		// we're done
		globals.networkError = '';
		await globals.loadCameras();
		replaceRoute({ name: "rtMonitor" });
		return;
	}

	stage.value++;
}

onMounted(async () => {
	let r = await fetchWithAuth("/api/auth/whoami");
	if (r.ok) {
		let ping = await (await fetchWithAuth("/api/ping")).json();
		if (ping.publicKey === '') {
			stage.value = Stages.SetupVPN;
		} else {
			let info = await (await fetchWithAuth("/api/system/info")).json();
			if (info.readyError) {
				stage.value = Stages.ConfigureVariables;
			} else {
				stage.value = Stages.ConfigureCameras;
			}
		}
	}

	// prime the network camera scanner
	// UPDATE: This is not needed, and complicates things by forcing scanNetworkForCameras to be an unprotected API.
	// Our network scan is so fast, that it's not necessary to warm it up.
	//fetchWithAuth("/api/config/scanNetworkForCameras", { method: "POST" });
})

</script>

<template>
	<mobile-fullscreen>
		<div class="flexColumnCenter">
			<h2 style="text-align: center; margin: 30px 10px">{{ stageText() }}</h2>
		</div>
		<new-user v-if="isCreateFirstUser" :is-first-user="true" @finished="moveToNextStage()" />
		<setup-v-p-n v-if="isSetupVPN" @finished="moveToNextStage()" />
		<system-variables v-if="isConfigureVariables" :initial-setup="true" @finished="moveToNextStage()" />
		<setup-cameras v-if="isConfigureCameras" @finished="moveToNextStage()" />
	</mobile-fullscreen>
</template>
