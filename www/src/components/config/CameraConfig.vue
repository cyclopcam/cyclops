<script setup lang="ts">
import type { CameraRecord } from '@/db/config/configdb';
import { ref } from 'vue';
import { Context } from '../form/forms';
import FormText from '../form/FormText.vue';
import FormBottom from '../form/FormBottom.vue';

let props = defineProps<{
	camera: CameraRecord
}>();

let ctx = new Context(() => true, { inputWidth: '140px' });
let host = ref(props.camera.host);
let username = ref(props.camera.username);
let password = ref(props.camera.password);

</script>

<template>
	<div>
		<form-text :ctx="ctx" label="IP Address / Hostname" v-model="host" />
		<div class="spacer" />
		<form-text :ctx="ctx" label="Username" v-model="username" />
		<div class="spacer" />
		<form-text :ctx="ctx" label="Password" v-model="password" :password="true" />
		<div class="spacer" />
		<form-bottom :ctx="ctx" submit-title="Test Connection" />
	</div>
</template>

<style lang="scss" scoped>
.spacer {
	height: 15px;
}
</style>
