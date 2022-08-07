<script setup lang="ts">
import type * as forms from './forms';
import Buttin from '@/components/core/Buttin.vue';

let props = defineProps<{
	ctx: forms.Context,
	submitTitle?: string,
}>()
let emit = defineEmits(['submit']);

function onSubmit() {
	props.ctx.submitClicked.value = true;
	if (props.ctx.validate())
		emit('submit');
}
</script>

<template>
	<div class="flexRowBaseline formBottom" style="align-self: stretch; justify-content: flex-end">
		<div v-if="ctx.showCompleteAllFields" class="required">Complete all required fields</div>
		<div v-if="ctx.submitError" class="submitError">{{ ctx.submitError.value }}</div>
		<div v-if="ctx.submitBusyMsg" class="submitBusy">{{ ctx.submitBusyMsg.value }}</div>
		<buttin :focal="true" :busy="ctx.busy.value" @click="onSubmit">{{ submitTitle ? submitTitle : 'Next' }}</buttin>
	</div>
</template>

<style lang="scss" scoped>
.formBottom {
	margin: 12px 8px;
}

.required {
	margin: 0 12px 0 0;
	font-size: 14px;
	color: #d00;
}

.submitError {
	margin: 0 12px 0 0;
	font-size: 14px;
	max-width: 250px;
	color: #d00;
}

.submitBusy {
	margin: 0 12px 0 0;
	font-size: 14px;
	max-width: 250px;
	color: #080;
}
</style>
