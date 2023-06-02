<script setup lang="ts">
import * as forms from '@/components/form/forms';
import FormText from '@/components/form/FormText.vue';
import FormBottom from '@/components/form/FormBottom.vue';
import { onMounted, ref } from 'vue';
import { encodeQuery, fetchOrErr, sleep } from '@/util/util';
import type { SystemInfoJSON } from '@/api/api';

let props = defineProps<{
	initialSetup?: boolean,
}>()
let emits = defineEmits(['finished']);

interface VariableDefinition {
	key: string;
	title: string;
	explanation: string;
	required: boolean;
	uiGroup: string;
}
interface VariableValue {
	key: string;
	value: string;
}
// Union of Definition and Value
interface Union {
	def: VariableDefinition;
	val: string;
}
// SYNC-SET-VARIABLE-RESPONSE
interface SetVariableResponse {
	wantRestart: boolean;
}

let existing = new Map<string, string>(); // value on server
let variables = ref([] as Union[]); // value in UI

let ctx = new forms.Context(() => {
	for (let v of variables.value) {
		if (v.def.required && v.val.trim() === '') {
			return false;
		}
	}
	return true;
},
	{
		inputWidth: '340px',
		inputColor: '#00a',
		submitTitle: props.initialSetup ? "Next" : "Save",
	}
);

async function onSubmit() {
	ctx.submitError.value = '';
	ctx.idToError.value = {};
	ctx.busy.value = true;
	let needRestart = false;
	let hasError = false;
	for (let v of variables.value) {
		if (v.val !== existing.get(v.def.key)) {
			// value has changed, so set on server
			let r = await fetchOrErr(`/api/config/setVariable/${v.def.key}?` + encodeQuery({ value: v.val || '' }), { method: "POST" });
			if (!r.ok) {
				ctx.idToError.value[v.def.key] = r.error;
				hasError = true;
				break;
			}
			let res = await r.r.json() as SetVariableResponse;
			if (res.wantRestart) {
				needRestart = true;
			}
		}
	}
	let mightBeReady = !needRestart && !hasError;
	if (!hasError && needRestart) {
		mightBeReady = await restart();
	}
	if (mightBeReady) {
		// If isReady fails, it will set ctx.submitError
		let ready = await isReady();
		if (ready) {
			emits('finished');
		}
	}
	ctx.busy.value = false;
}

async function restart(): Promise<boolean> {
	console.log("Starting restart");
	let r = await fetchOrErr('/api/system/restart', { method: 'POST' });
	if (r.ok) {
		console.log("Restart in progress");
		ctx.submitBusyMsg.value = "Restarting";
		return true;
	} else {
		console.log("Restart failed");
		ctx.submitError.value = r.error;
		return false;
	}
}

async function isReady(): Promise<boolean> {
	// Before we even start, wait for 500 milliseconds. This is here to try and
	// make restarts work with the vite devserver, which freaks out when the Go server
	// goes down, and doesn't seem to realize that it's back up again.
	await sleep(500);

	// Keep retrying, because isReady is intended to be run after restarting the server
	let timeoutMS = 3000;
	let start = new Date().getTime();
	let connectError = '';
	for (let attempt = 0; new Date().getTime() - start < timeoutMS; attempt++) {
		let r = await fetchOrErr('/api/system/info');
		if (r.ok) {
			let j = await r.r.json() as SystemInfoJSON;
			if (j.readyError) {
				console.log(`isReady: readyError = ${j.readyError}`);
				ctx.submitError.value = j.readyError;
				return false;
			} else {
				console.log('Ready!');
				return true;
			}
		} else {
			console.log('isReady sleeping');
			connectError = r.error;
			await sleep(300);
		}
	}
	console.log(`isReady timeout. Last error was ${connectError}`);
	ctx.submitError.value = connectError;
	return false;
}

async function fetchLatest() {
	let definitions = await (await fetch('/api/config/getVariableDefinitions')).json() as VariableDefinition[];
	definitions.sort((a, b) => a.uiGroup.localeCompare(b.uiGroup));
	let val = await (await fetch('/api/config/getVariableValues')).json() as VariableValue[];
	existing = new Map<string, string>();
	let vmap: any = {};
	for (let v of val) {
		vmap[v.key] = v.value;
	}
	for (let def of definitions) {
		let value = vmap[def.key] || '';
		existing.set(def.key, value);
		variables.value.push({
			def: def,
			val: value,
		});
	}
}

onMounted(() => {
	fetchLatest();
})

</script>

<template>
	<div class="flexColumn">
		<div v-for="v of variables">
			<form-text :ctx="ctx" :id="v.def.key" :big-label="v.def.explanation" :label="v.def.title" v-model="v.val"
				:required="v.def.required" placeholder="/..." />
		</div>
		<form-bottom :ctx="ctx" @submit="onSubmit" />
	</div>
</template>

<style lang="scss" scoped></style>
