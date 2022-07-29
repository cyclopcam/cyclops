<script setup lang="ts">
import MobileFullscreen from '@/components/responsive/MobileFullscreen.vue';
import NewUser from '@/components/config/NewUser.vue';
import { onMounted, ref } from 'vue';
import { computed } from '@vue/reactivity';
import CamerasConfig from '../components/config/CamerasConfig.vue';

enum Stages {
	CreateFirstUser = 0,
	ConfigureCameras = 1,
}

let stage = ref(Stages.CreateFirstUser);
let isCreateFirstUser = computed(() => stage.value === Stages.CreateFirstUser);
let isConfigureCameras = computed(() => stage.value === Stages.ConfigureCameras);

function stageText() {
	switch (stage.value) {
		case Stages.CreateFirstUser:
			return "Create a username and password for yourself";
		case Stages.ConfigureCameras:
			return "Let's find your cameras";
	}
}

function moveToNextStage() {
	//console.log("moveToNextStage");
	stage.value++;
}

onMounted(async () => {
	let r = await fetch("/api/auth/whoami");
	if (r.ok) {
		stage.value = Stages.ConfigureCameras;
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
		<cameras-config v-if="isConfigureCameras" />
	</mobile-fullscreen>
</template>
