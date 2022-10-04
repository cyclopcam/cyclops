<script setup lang="ts">
import type { ParsedServer } from '@/scan';
import { onMounted, ref } from 'vue';
import { encodeQuery } from '@/util/util';
import { ServerPort } from '@/global';

let serverPort = ServerPort;

let props = defineProps<{
	ip: string,
	host: string,
}>()

// SYNC-CONNECT-RESPONSE
//interface ConnectResponse {
//	state: "n" | "o" | "e"; // new, old, error
//	error: "";
//}

let isNew = ref(false);
let error = ref("");

onMounted(async () => {
	// do we need this page? It's nice for the swipe transitions... and Back() ability
	let baseUrl = `http://${props.ip}:${serverPort}`;
	try {
		let r = await (await fetch('/natcom/forward?' + encodeQuery({ url: baseUrl + "/api/auth/hasAdmin" }))).json();
		isNew.value = r === false;
		await fetch('/natcom/showServer?' + encodeQuery({ url: baseUrl }));
	} catch (e) {
		error.value = e + "";
	}

})

</script>
 
<template>
	<div>
		<h3>Attemping connection to</h3>
		<h2>{{ip}} ({{host}})</h2>
		<div class="block">
			<div v-if="error" class="error">{{error}}</div>
			<div v-else>{{isNew ? "New server" : "Old server" }}</div>
		</div>
	</div>
</template>

<style scoped lang="scss">
.error {
	color: #d00;
}

.block {
	margin: 10px 20px;

}
</style>
