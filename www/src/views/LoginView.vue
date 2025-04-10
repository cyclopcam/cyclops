<script setup lang="ts">
import MobileFullscreen from '@/components/responsive/MobileFullscreen.vue';
import { onMounted, ref } from 'vue';
import * as forms from '@/components/form/forms';
import FormText from '@/components/form/FormText.vue';
import FormBottom from '@/components/form/FormBottom.vue';
import { globals } from '@/globals';
import { login } from '@/auth';

// WARNING! This has been subsumed by WelcomeView.vue
// Will delete soon.

let username = ref("");
let password = ref("");

let ctx = new forms.Context(() =>
	username.value.trim() !== '' && password.value.trim() !== ''
);

async function onSubmit() {
	ctx.submitError.value = '';

	ctx.busy.value = true;
	let loginError = await login(username.value, password.value);
	ctx.busy.value = false;
	if (loginError !== '') {
		ctx.submitError.value = loginError;
		return;
	}

	// Just reload to make sure all state is sane, and we go through
	// whatever automatic routing the default system would take us
	// through on app startup.
	window.location.reload();

	// This approach is just trying too hard - not worth it.
	//globals.networkError = '';
	//globals.postAuthenticateLoadSystemInfo(true);
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

<style lang="scss" scoped></style>
