<script setup lang="ts">
import MobileFullscreen from '@/components/responsive/MobileFullscreen.vue';
import { onMounted, ref } from 'vue';
import * as forms from '@/components/form/forms';
import FormText from '@/components/form/FormText.vue';
import FormBottom from '@/components/form/FormBottom.vue';
import { encodeQuery, fetchOrErr } from '@/util/util';
import { globals } from '@/globals.js';

let username = ref("");
let password = ref("");

let ctx = new forms.Context(() =>
	username.value.trim() !== '' && password.value.trim() !== ''
);

async function onSubmit() {
	ctx.submitError.value = '';
	let basic = btoa(username.value.trim() + ":" + password.value.trim());
	ctx.busy.value = true;
	let r = await fetchOrErr('/api/auth/login?' + encodeQuery({ loginMode: "BearerToken" }), { method: 'POST', headers: { "Authorization": "BASIC " + basic } });

	ctx.busy.value = false;
	if (!r.ok) {
		ctx.submitError.value = r.error;
		return;
	}

	let j = await r.r.json();
	let token = j.bearerToken;

	// Save our token to localStorage.
	localStorage.setItem("bearerToken", token);

	// Inform our mobile app that we've logged in. Chrome's limit on cookie duration is about 400 days,
	// but we can extend that by not using cookies. Also, the mobile app needs to know the list of
	// servers that the client knows about.
	fetch('/natcom/login?' + encodeQuery({ bearerToken: token }));

	globals.networkError = '';
	globals.postLoadAutoRoute();
}

//onMounted(() => {
//	ctx.busy.value = true;
//})

</script>

<template>
	<mobile-fullscreen>
		<div class="flexColumnCenter">
			<h2 style="text-align: center; margin: 30px 10px">Login</h2>

			<form-text :ctx="ctx" v-model="username" placeholder="username" :required="true" :focus="true"
				autocomplete="username" />
			<form-text :ctx="ctx" v-model="password" placeholder="password" :required="true" :password="true"
				:submit-on-enter="true" />
			<form-bottom :ctx="ctx" submit-title="Login" @submit="onSubmit" />
		</div>
	</mobile-fullscreen>
</template>

<style lang="scss" scoped>

</style>
