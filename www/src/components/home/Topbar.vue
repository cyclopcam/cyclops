<script setup lang="ts">
import ToggleButton from "../core/ToggleButton.vue";
import settings from "@/icons/settings.svg";
import monitor from "@/icons/monitor.svg";
import { onMounted } from "vue";
import router from "@/router/routes";

onMounted(() => {
	//console.log("Route", router.currentRoute.value.name);
})

</script>

<template>
	<div class="flex topbar">
		<toggle-button :icon="settings" title="Settings" route="rtSettings" />
		<toggle-button :icon="monitor" title="Monitor" route="rtMonitor" />
	</div>
</template>

<style lang="scss" scoped>
@import '@/assets/vars.scss';

.topbar {
	justify-content: center;
	gap: 12px;
	margin: 4px 4px 4px 4px;
	//@media (max-width: $mobileCutoff) {
	//	margin: 10px;
	//}
}
</style>
