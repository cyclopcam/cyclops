<script setup lang="ts">
import ToggleButton from "../core/ToggleButton.vue";
import { onMounted } from "vue";
import settings from "@/icons/settings.svg";
import monitor from "@/icons/monitor.svg";
import bulb from "@/icons/bulb.svg";

onMounted(() => {
	//console.log("Route", router.currentRoute.value.name);
})

</script>

<template>
	<div class="flex topbar">
		<toggle-button :icon="settings" title="Settings" route="rtSettings" />
		<toggle-button :icon="monitor" title="Monitor" route="rtMonitor" />
		<toggle-button :icon="bulb" title="Train" route="rtTrain" />
	</div>
</template>

<style lang="scss" scoped>
@import '@/assets/vars.scss';

.topbar {
	justify-content: center;
	gap: 12px;
	padding: 6px 4px 6px 4px;
	box-shadow: 0px 1px 3px rgba(0, 0, 0, 0.1);

	@media (max-width: $mobileCutoff) {
		padding: 4px 4px 4px 4px;
	}
}
</style>
