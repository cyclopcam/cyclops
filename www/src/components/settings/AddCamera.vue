<script setup lang="ts">
import { useRouter } from 'vue-router';
import { pushRoute } from "@/router/helpers";
import WideButton from '@/components/widewidgets/WideButton.vue';
import TopologyStar3 from '@/icons/topology-star-3.svg';
import TextPlus from '@/icons/text-plus.svg';

const router = useRouter();

function onManual() {
	// Params are not allowed to be empty strings, so we use a space as a placeholder.
	// The edit camera component recognizes this and treats it like an empty string.
	pushRoute(router, { name: 'rtSettingsEditCamera', params: { id: 'new', host: ' ', model: ' ', returnToScan: 0 } });
}

function onScan() {
	pushRoute(router, { name: 'rtSettingsScanForCameras', params: { usePreviousScan: '0' } });
}

</script>

<template>
	<div class="wideRoot">
		<wide-button :icon="TopologyStar3" @click="onScan">Scan local network for cameras</wide-button>
		<wide-button :icon="TextPlus" @click="onManual">Enter camera details manually</wide-button>
	</div>
</template>

<style lang="scss" scoped></style>
