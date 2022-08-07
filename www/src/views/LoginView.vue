<script setup lang="ts">
import MobileFullscreen from '@/components/responsive/MobileFullscreen.vue';
import { ref } from 'vue';
import * as forms from '@/components/form/forms';
import FormText from '@/components/form/FormText.vue';
import FormBottom from '@/components/form/FormBottom.vue';
import { fetchOrErr } from '@/util/util';
import { globals } from '@/globals.js';

let username = ref("");
let password = ref("");

let ctx = new forms.Context(() =>
	username.value.trim() !== '' && password.value.trim() !== ''
);

async function onSubmit() {
	ctx.submitError.value = '';
	let basic = btoa(username.value + ":" + password.value);
	let r = await fetchOrErr('/api/auth/login', { method: 'POST', headers: { "Authorization": "BASIC " + basic } });
	if (!r.ok) {
		ctx.submitError.value = r.error;
		return;
	}
	globals.postLoginAutoRoute();
}

</script>

<template>
	<mobile-fullscreen>
		<div class="flexColumnCenter">
			<h2 style="text-align: center; margin: 30px 10px">Login</h2>

			<form-text :ctx="ctx" v-model="username" placeholder="username" :required="true" :focus="true"
				autocomplete="username" />
			<form-text :ctx="ctx" v-model="password" placeholder="password" :required="true" :password="true" />
			<form-bottom :ctx="ctx" submit-title="Login" @submit="onSubmit" />
		</div>
	</mobile-fullscreen>
</template>

<style lang="scss" scoped>
</style>
