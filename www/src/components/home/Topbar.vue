<script setup lang="ts">
import ToggleButton from "../core/ToggleButton.vue";
import { onMounted } from "vue";
import settings from "@/icons/settings.svg";
import monitor from "@/icons/monitor.svg";
import bulb from "@/icons/bulb.svg";
import back from "@/icons/back.svg";
import { globals } from "@/globals";
import { computed } from "@vue/reactivity";
import SvgButton from "../core/SvgButton.vue";
import router from "@/router/routes";

let error = computed(() => {
	return globals.networkError;
})

function onBack() {
	// We probably want to pop one up in the hierarchy, instead of going back.
	router.back();
}

function showBack(): boolean {
	// Don't count paths which are the default child.
	// The default child paths are the same as their parent,
	// so that's how we detect them.
	let d = 0;
	let previous = '';
	for (let p of router.currentRoute.value.matched) {
		if (p.path !== previous) {
			d++;
			previous = p.path;
		}
	}
	return d >= 3;
}

onMounted(() => {
	//console.log("Route", router.currentRoute.value);
})

</script>

<template>
	<div class="topbarOuter">
		<div class="topbarInner">
			<div class="flex" style="width: 60px">
				<svg-button v-if="showBack()" :icon="back" icon-size="28px" style="margin-left:10px" @click="onBack" />
			</div>
			<div class="centerGroup">
				<toggle-button :icon="settings" title="Settings" route="rtSettings" route-target="rtSettingsTop" />
				<toggle-button :icon="monitor" title="Monitor" route="rtMonitor" />
				<toggle-button :icon="bulb" title="Train" route="rtTrain" route-target="rtTrainHome" />
			</div>
			<div class="flex" style="width: 60px">
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

	width: 360px;

	@media (max-width: $mobileCutoff) {
		width: 100vw;
	}

	@media (max-width: $mobileCutoff) {
		padding: 4px 4px 4px 4px;
	}
}

.centerGroup {
	display: flex;
	gap: 8px;
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
