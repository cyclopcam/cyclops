<script setup lang="ts">
import MobileFullscreen from '@/components/responsive/MobileFullscreen.vue';
import NewUser from '@/components/settings/NewUser.vue';
import { onMounted, ref } from 'vue';
import { computed } from '@vue/reactivity';
import SetupCameras from '../components/settings/SetupCameras.vue';
import { globals } from '@/globals';
import router from "@/router/routes";
import SystemVariables from "../components/settings/SystemVariables.vue";

enum Stages {
	CreateFirstUser = 0,
	ConfigureVariables = 1,
	ConfigureCameras = 2,
}

let stage = ref(Stages.CreateFirstUser);
let isCreateFirstUser = computed(() => stage.value === Stages.CreateFirstUser);
let isConfigureVariables = computed(() => stage.value === Stages.ConfigureVariables);
let isConfigureCameras = computed(() => stage.value === Stages.ConfigureCameras);

function stageText() {
	switch (stage.value) {
		case Stages.CreateFirstUser:
			return "Create a username and password for yourself";
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
		router.replace({ name: "rtMonitor" });
		return;
	}

	stage.value++;
}

onMounted(async () => {
	let r = await fetch("/api/auth/whoami");
	if (r.ok) {
		let info = await (await fetch("/api/system/info")).json();
		if (info.readyError) {
			stage.value = Stages.ConfigureVariables;
		} else {
			stage.value = Stages.ConfigureCameras;
		}
	}

	// prime the network camera scanner
	fetch("/api/config/scanNetworkForCameras", { method: "POST" });
})

</script>

<template>
	<mobile-fullscreen>
		<div class="flexColumnCenter">
			<h2 style="text-align: center; margin: 30px 10px">{{ stageText() }}</h2>
		</div>
		<new-user v-if="isCreateFirstUser" :is-first-user="true" @finished="moveToNextStage()" />
		<system-variables v-if="isConfigureVariables" :initial-setup="true" @finished="moveToNextStage()" />
		<setup-cameras v-if="isConfigureCameras" @finished="moveToNextStage()" />
	</mobile-fullscreen>
</template>
