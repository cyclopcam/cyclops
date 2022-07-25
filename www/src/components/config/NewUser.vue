<script setup lang="ts">
import * as forms from '@/components/form/forms';
import FormText from '@/components/form/FormText.vue';
import FormBottom from '@/components/form/FormBottom.vue';
import { ref } from 'vue';

let props = defineProps<{
	isFirstUser: boolean,
}>()

let username = ref("");
let password = ref("");
let missing = ref(false);

let ctx = new forms.Context(() =>
	username.value.trim() !== '' && password.value.trim() !== ''
);

function onNext() {
}

</script>

<template>
	<div class="flexColumnCenter">
		<form-text :ctx="ctx" v-model="username" placeholder="username" :required="true" />
		<form-text :ctx="ctx" v-model="password" placeholder="password" :required="true" :password="true" />
		<form-bottom :ctx="ctx" :missing="missing" @next="onNext" />
	</div>
</template>

<style lang="scss" scoped>
</style>