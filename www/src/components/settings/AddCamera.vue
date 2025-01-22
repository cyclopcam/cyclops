<script setup lang="ts">
import { useRouter } from 'vue-router';
import { pushRoute } from "@/router/helpers";
import WideRoot from '@/components/widewidgets/WideRoot.vue';
import WideButton from '@/components/widewidgets/WideButton.vue';
import WideSection from '@/components/widewidgets/WideSection.vue';
import TopologyStar3 from '@/icons/topology-star-3.svg';
import TextPlus from '@/icons/text-plus.svg';

const router = useRouter();

function onManual() {
	// Params are not allowed to be empty strings, so we use a space as a placeholder.
	// The edit camera component recognizes this and treats it like an empty string.
	pushRoute(router, { name: 'rtSettingsEditCamera', params: { id: 'new' }, hash: 'returnToScan:0' });
}

function onScan() {
	pushRoute(router, { name: 'rtSettingsScanForCameras' });
}

</script>

<template>
	<wide-root title="Add Camera">
		<wide-section>
			<wide-button :icon="TopologyStar3" :right-arrow="true" @click="onScan">Scan local network for
				cameras</wide-button>
			<wide-button :icon="TextPlus" :right-arrow="true" @click="onManual">Enter camera details
				manually</wide-button>
		</wide-section>
	</wide-root>
</template>

<style lang="scss" scoped></style>
