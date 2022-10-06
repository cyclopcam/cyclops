<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { fetchOrErr, sleep } from '@/util/util';

let emits = defineEmits(['finished']);

let busy = ref(true);
let error = ref('');
let showWarning = ref(false);

function isGood(): boolean {
	return !busy.value && error.value === '';
}

async function onTryAgain() {
	start(600);
}

let nTries = 0;
let mockSuccessAfter2Tries = false; // This is for mocking the UI, so we can play with the failed connection UX

async function start(pauseMS = 0) {
	let startedAt = new Date().getTime();
	busy.value = true;
	error.value = '';
	let r = await fetchOrErr('/api/system/startVPN', { method: 'POST' });

	// this is just to let the user know that we DID ACTUALLY TRY, because failure can be very fast
	let elapsedMS = new Date().getTime() - startedAt;
	let remainingPauseMS = pauseMS - elapsedMS;
	if (remainingPauseMS > 0) {
		await sleep(remainingPauseMS);
	}

	busy.value = false;
	nTries++;
	if (mockSuccessAfter2Tries && nTries === 2) {
		return;
	}

	if (!r.ok) {
		// connection failure
		error.value = r.error;
	} else {
		// response received
		let j = await r.r.json();
		error.value = j.error;
	}
}

function onNext() {
	if (isGood() || showWarning.value) {
		emits('finished');
	} else {
		showWarning.value = true;
	}
}

onMounted(async () => {
	start();
})

</script>
	
<template>
	<div class="flexColumnCenter">
		<h3 v-if="busy">Attempting to connect...</h3>
		<div v-if="!isGood()" class="instructions">The service <code>cyclopskernelwg</code>
			must be running.<br /><br />
			To start the service, login to your device terminal and run:
			<pre><code>sudo systemctl start cyclopskernelwg</code></pre>
		</div>
		<h3 v-if="isGood()" class="success">Success!</h3>
		<div v-if="error" class="error">{{error}}</div>
		<div v-if="showWarning" class="flex warning">
			If you do not activate VPN functionality, you will be unable to access your Cyclops system
			from outside your Wifi network.
		</div>
		<div class="flex bottom">
			<button :class="{focalButton:!isGood()}" v-if="!isGood()" @click="onTryAgain()" :disabled="busy"
				style="margin-right: 20px">Try
				Again</button>
			<button :class="{focalButton:isGood(), dangerButton:showWarning}" @click="onNext()">Next</button>
		</div>
	</div>
</template>
	
<style lang="scss" scoped>
.instructions {
	margin: 10px;
}

.success {
	color: #080;
}

.error {
	margin: 10px;
	color: #d00;
}

.bottom {
	margin: 20px 10px 10px 10px;
	align-self: stretch;
	justify-content: flex-end;
}

.warning {
	margin: 10px;
	color: rgb(194, 113, 0);
	align-self: stretch;
	justify-content: flex-end;
}
</style>