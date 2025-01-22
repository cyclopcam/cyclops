<script setup lang="ts">
import WideSection from '@/components/widewidgets/WideSection.vue';
import WideRoot from '@/components/widewidgets/WideRoot.vue';
import WideButton from '@/components/widewidgets/WideButton.vue';
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
	<wide-root>
		<wide-section>
			<wide-button :icon="server" route-target="rtSettingsSystem">System Settings</wide-button>
			<wide-button v-for="camera of configured" :route-target="`/settings/camera/${camera.id}`"
				:icon="camera.posterURL()" icon-size="45px">
				{{ camera.name }}</wide-button>
			<wide-button route-target="rtSettingsAddCamera" :icon="addIcon">Add
				Camera</wide-button>
		</wide-section>
	</wide-root>
</template>
