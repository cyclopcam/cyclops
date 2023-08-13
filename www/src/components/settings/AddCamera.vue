<script setup lang="ts">
import { useRouter } from 'vue-router';
import { pushRoute } from "@/router/helpers";
import WideButton from '@/components/widewidgets/WideButton.vue';
import TopologyStar3 from '@/icons/topology-star-3.svg';
import TextPlus from '@/icons/text-plus.svg';

const router = useRouter();

function onManual() {
	pushRoute(router, { name: 'rtSettingsEditCamera', params: { id: 'new' } });
}

function onScan() {
	pushRoute(router, { name: 'rtSettingsScanForCameras' });
}

</script>

<template>
	<div class="wideRoot">
		<wide-button :icon="TopologyStar3" @click="onScan">Scan local network for cameras</wide-button>
		<wide-button :icon="TextPlus" @click="onManual">Enter camera details manually</wide-button>
	</div>
</template>

<style lang="scss" scoped></style>
