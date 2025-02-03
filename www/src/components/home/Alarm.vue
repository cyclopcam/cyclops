<script setup lang="ts">
import Toggle from '@/components/widgets/Toggle.vue';
import { fetchOrErr } from '@/util/util';
import { onMounted, ref } from 'vue';

let isToggled = ref(false);
let isTriggered = ref(false);

async function onArm(armed: boolean) {
	let r = await fetchOrErr(armed ? '/api/system/alarm/arm' : '/api/system/alarm/disarm', { method: 'POST' });
	if (!r.ok) {
		console.log(r.error);
	} else {
		//isToggled.value = armed;
	}
}

onMounted(async () => {
	let r = await fetchOrErr('/api/system/alarm/status');
	if (!r.ok) {
		console.log(r.error);
	} else {
		let j = await r.r.json();
		isToggled.value = j.armed;
		isTriggered.value = j.triggered;
	}
});

</script>

<template>
	<div class="flexColumn alarmRoot">
		<div class="flexCenter">
			<div style="margin-right: 20px">Arm</div>
			<toggle v-model="isToggled" @change="onArm" />
		</div>
		<div class="flexCenter" style="margin-top: 50px">
			<div :class="{ status: true, triggered: isTriggered }">{{ isTriggered ? 'Alarm!' : 'Status: OK' }}</div>
		</div>
	</div>
</template>

<style lang="scss" scoped>
@import '@/assets/base.scss';

.alarmRoot {
	box-sizing: border-box;
	background: $widerootbg;
	height: 100%;

	// We do this, otherwise the invocation of a vertical scrollbar on desktop causes
	// there to be too little horizontal space, and then we get a really ugly horizontal
	// scrollbar too.
	overflow-x: hidden;

	overflow-y: auto;

	// for desktop
	width: 420px;

	// for mobile
	@media (max-width: $mobileCutoff) {
		width: 100%;
	}

	align-items: center;
	justify-content: center;
}

.status {}

.triggered {
	font-weight: bold;
	color: #e00;
}
</style>
