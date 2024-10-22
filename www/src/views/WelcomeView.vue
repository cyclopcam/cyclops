<script setup lang="ts">
import MobileFullscreen from '@/components/responsive/MobileFullscreen.vue';
import NewUser from '@/components/settings/NewUser.vue';
import SetupVPN from '@/components/settings/SetupVPN.vue';
import { onMounted, ref } from 'vue';
import { computed } from '@vue/reactivity';
import SetupCameras from '../components/settings/SetupCameras.vue';
import { globals } from '@/globals';
import { replaceRoute } from "@/router/helpers";
import { useRouter } from 'vue-router';

const router = useRouter();

enum Stages {
	SetupVPN = 0, // This is disabled.. but maybe someday we bring it back.
	CreateFirstUser = 1,
	//ConfigureVariables = 2,
	ConfigureCameras = 2,
}

let stage = ref(Stages.CreateFirstUser);
let isSetupVPN = computed(() => stage.value === Stages.SetupVPN);
let isCreateFirstUser = computed(() => stage.value === Stages.CreateFirstUser);
//let isConfigureVariables = computed(() => stage.value === Stages.ConfigureVariables);
let isConfigureCameras = computed(() => stage.value === Stages.ConfigureCameras);

function stageText() {
	switch (stage.value) {
		case Stages.SetupVPN:
			return "VPN Activation";
		case Stages.CreateFirstUser:
			return "Create a username and password for yourself";
		//case Stages.ConfigureVariables:
		//	return "System configuration";
		case Stages.ConfigureCameras:
			return "Let's find your cameras";
	}
}

async function moveToNextStage() {
	console.log("moveToNextStage");
	await globals.postAuthenticateLoadSystemInfo(false);
	replaceRoute(router, { name: "rtSettingsHome" });


	/*
	if (stage.value === Stages.ConfigureCameras) {
		// we're done
		globals.networkError = '';
		await globals.loadCameras();
		replaceRoute(router, { name: "rtMonitor" });
		return;
	}

	stage.value++;

	if (stage.value === Stages.CreateFirstUser) {
		// This code path is necessary for when the VPN is still not setup, but the user is logged in
		let r = await fetch("/api/auth/whoami");
		if (r.ok) {
			moveToNextStage();
		}
	}
	*/
}

onMounted(async () => {
	await globals.waitForPublicKeyLoad();

	await globals.waitForSystemInfoLoad();
	if (!globals.isLoggedIn) {
		stage.value = Stages.CreateFirstUser;
		return;
	}

	stage.value = Stages.ConfigureCameras;
})

</script>

<template>
	<mobile-fullscreen>
		<div class="flexColumnCenter">
			<h2 style="text-align: center; margin: 30px 10px">{{ stageText() }}</h2>
		</div>
		<setup-v-p-n v-if="isSetupVPN" @finished="moveToNextStage()" />
		<new-user v-if="isCreateFirstUser" :is-first-user="true" @finished="moveToNextStage()" />
		<setup-cameras v-if="isConfigureCameras" :isInitialSetup="true" @finished="moveToNextStage()" />
	</mobile-fullscreen>
</template>
