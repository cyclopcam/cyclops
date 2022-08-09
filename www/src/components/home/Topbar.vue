<script setup lang="ts">
import ToggleButton from "../core/ToggleButton.vue";
import { onMounted } from "vue";
import settings from "@/icons/settings.svg";
import monitor from "@/icons/monitor.svg";
import bulb from "@/icons/bulb.svg";
import { globals } from "@/globals";
import { computed } from "@vue/reactivity";

let error = computed(() => {
	return globals.networkError;
})

onMounted(() => {
	//console.log("Route", router.currentRoute.value.name);
})

</script>

<template>
	<div class="topbarOuter">
		<div class="topbarInner">
			<div class="flex">
				<!-- maybe a back button or something -->
			</div>
			<div class="centerGroup">
				<toggle-button :icon="settings" title="Settings" route="rtSettings" />
				<toggle-button :icon="monitor" title="Monitor" route="rtMonitor" />
				<toggle-button :icon="bulb" title="Train" route="rtTrain" route-target="rtTrainManageRecordings" />
			</div>
			<div class="flex">
				<!-- logout or something -->
			</div>
		</div>
		<div v-if="error" class="error">
			{{ error }}
		</div>
	</div>
</template>

<style lang="scss" scoped>
@import '@/assets/vars.scss';

.topbarOuter {
	display: flex;
	flex-direction: column;
	align-items: center;

	padding: 6px 4px 6px 4px;

	@media (max-width: $mobileCutoff) {
		padding: 4px 4px 4px 4px;
	}

	box-shadow: 0px 1px 3px rgba(0, 0, 0, 0.1);
}

.topbarInner {
	display: flex;
	justify-content: space-between;
	max-width: 400px;

	@media (max-width: $mobileCutoff) {
		padding: 4px 4px 4px 4px;
	}
}

.centerGroup {
	display: flex;
	gap: 12px;
}

.error {
	font-size: 14px;
	margin: 8px 2px 0px 2px;
	color: #c00;
	border-radius: 3px;
	background-color: rgb(255, 252, 222);
	border: solid 1px rgb(255, 231, 125);
	padding: 5px 8px;
	max-width: 90vw;
}
</style>
