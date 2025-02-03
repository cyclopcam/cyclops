<script setup lang="ts">
import { fetchOrErr, sleep } from '@/util/util';
import { onMounted, ref } from 'vue';
import shield_filled from "@/icons/shield-filled.svg";
import shield_off from "@/icons/shield-off.svg";
import spinner from "@/icons/loaders/circle-spinner.svg";

let isBusy = ref(false);
let isArmed = ref(false);
let isTriggered = ref(false);
let error = ref('');

async function onArm(armed: boolean) {
	isBusy.value = true;
	let r = await fetchOrErr(armed ? '/api/system/alarm/arm' : '/api/system/alarm/disarm', { method: 'POST' });
	isBusy.value = false;
	if (!r.ok) {
		error.value = r.error;
	} else {
		error.value = '';
		isArmed.value = armed;
		if (!armed) {
			isTriggered.value = false;
		}
	}
}

function statusImage(): any {
	if (isBusy.value) {
		return spinner;
	} else if (isTriggered.value) {
		return shield_filled;
	} else {
		return isArmed.value ? shield_filled : shield_off;
	}
}

onMounted(async () => {
	isBusy.value = true;
	let r = await fetchOrErr('/api/system/alarm/status');
	isBusy.value = false;
	if (!r.ok) {
		error.value = r.error;
	} else {
		let j = await r.r.json();
		isArmed.value = j.armed;
		isTriggered.value = j.triggered;
	}
});

</script>

<template>
	<div class="flexColumn wideRootInner alarmRoot">
		<div class="flexCenter flexColumn" style="margin-bottom: 20px">
			<img :src="statusImage()" :class="{ status: true, orangeTint: isArmed }" />
			<div :class="{ armed: isArmed }">{{ isArmed ? 'Armed' : 'Not Armed' }}</div>
			<div :class="{ triggered: isTriggered }">{{ isTriggered ? 'Alarm!' : '' }}</div>
		</div>
		<div class="errorSection">
			<div v-if="error" class="error">{{ error }}</div>
		</div>
		<div class="flexCenter buttonRow">
			<button class="mainBtn" style="margin-right: 40px" @click="onArm(false)">
				<img :src="shield_off" />
				Disarm
			</button>
			<button class="mainBtn" @click="onArm(true)">
				<img :src="shield_filled" />
				Arm
			</button>
		</div>
	</div>
</template>

<style lang="scss" scoped>
@import '@/assets/base.scss';

.alarmRoot {
	align-items: center;
	justify-content: center;
}

.status {
	width: 40px;
	height: 40px;
	margin-bottom: 15px;
}

.orangeTint {
	filter: invert(0.5) sepia(1) brightness(0.8) saturate(5) hue-rotate(-30deg);
}

.errorSection {
	height: 30px;
}

.error {
	color: #d00;
}

.buttonRow {
	margin-top: 50px;
}

.mainBtn {
	display: flex;
	flex-direction: column;
	align-items: center;
	border-radius: 25px;
	padding: 20px 30px;
	//width: 90px;
	-webkit-tap-highlight-color: transparent;
	touch-action: manipulation;
}

.mainBtn img {
	width: 60px;
	height: 60px;
	margin-bottom: 20px;
}

.triggered {
	font-weight: bold;
	color: #e00;
}
</style>
