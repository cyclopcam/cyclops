<script setup lang="ts">
import { ref } from 'vue';
import { fetchOrErr, parseUrlHash } from '@/util/util';

let username = ref("");
let password = ref("");
let error = ref("");

async function onLogin() {
	let basic = btoa(username.value.trim() + ":" + password.value.trim());
	let r = await fetchOrErr('/api/auth/login', { method: 'POST', headers: { "Authorization": "BASIC " + basic } });
	if (!r.ok) {
		error.value = r.error;
		return;
	}
	error.value = '';
	let hash = parseUrlHash();
	if (hash['redirectTo']) {
		window.location.href = hash['redirectTo'];
	} else {
		window.location.href = "/";
	}
}

</script>

<template>
	<div class="centered" style="height:100%">
		<form class="panel flexColumn" @submit.prevent="onLogin">
			<div class="formLine flexRow">
				<label class="flexRow">
					<div class="field">Username</div>
					<input v-model="username" />
				</label>
			</div>
			<div class="formLine flexRow">
				<label class="flexRow">
					<div class="field">Password</div>
					<input v-model="password" type="password" />
				</label>
			</div>
			<div class="error" v-if="error !== ''" style="margin: 10px 5px">
				{{ error }}
			</div>
			<div class="flexRow" style="justify-content: flex-end; margin-top: 15px">
				<button type="submit">Login</button>
			</div>
		</form>
	</div>
</template>


<style scoped>
.formLine {
	margin: 5px;
}

.field {
	width: 100px;
}
</style>
