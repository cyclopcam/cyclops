<script setup lang="ts">
import PanelButton from "../core/PanelButton.vue";
import Panel from "../core/Panel.vue";
import server from "@/icons/server.svg";
import { onMounted, ref } from 'vue';
import { CameraRecord } from '@/db/config/configdb';
import { fetchOrErr } from "@/util/util";
import addIcon from "@/icons/plus-circle.svg";

let configured = ref([] as CameraRecord[]); // cameras in the server DB

async function fetchExisting() {
	let r = await fetchOrErr('/api/config/cameras');
	if (r.ok) {
		configured.value = ((await r.r.json()) as []).map(x => CameraRecord.fromJSON(x));
	}
}

onMounted(async () => {
	await fetchExisting();
})

</script>

<template>
	<panel>
		<panel-button :icon="server" route-target="rtSettingsSystem">System Settings</panel-button>
		<panel-button v-for="camera of configured" :route-target="`/settings/camera/${camera.id}`"
			:icon="camera.posterURL()" icon-size="50px">
			{{ camera.name }}</panel-button>
		<panel-button route-target="rtSettingsAddCamera" :icon="addIcon">Add
			Camera</panel-button>
	</panel>
</template>
