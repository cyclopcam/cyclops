<script setup lang="ts">
import * as forms from '@/components/form/forms';
import FormText from '@/components/form/FormText.vue';
import FormBottom from '@/components/form/FormBottom.vue';
import { ref } from 'vue';
import { Permissions, UserRecord } from '@/db/config/configdb';
import { encodeQuery, fetchOrErr } from '@/util/util';

let props = defineProps<{
	isFirstUser: boolean,
}>()
let emits = defineEmits(['finished']);

let username = ref("");
let password = ref("");

let ctx = new forms.Context(() =>
	username.value.trim() !== '' && password.value.trim() !== ''
);

async function onSubmit() {
	let user = new UserRecord();
	user.username = username.value;
	user.permissions = Permissions.Admin;
	ctx.busy.value = true;
	let r = await fetchOrErr('/api/auth/createUser?' + encodeQuery({ password: password.value }), { method: 'POST', body: JSON.stringify(user.toJSON()) });
	ctx.busy.value = false;
	if (!r.ok) {
		ctx.submitError.value = r.error;
		return;
	}
	// The initial createUser also logs us in, so we don't need to worry about logging in
	emits('finished');
}

</script>

<template>
	<div class="flexColumnCenter">
		<form-text :ctx="ctx" v-model="username" placeholder="username" :required="true" :focus="true" />
		<form-text :ctx="ctx" v-model="password" placeholder="password" :required="true" :password="true" />
		<form-bottom :ctx="ctx" @submit="onSubmit" />
	</div>
</template>

<style lang="scss" scoped>
</style>