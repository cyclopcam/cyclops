<script setup lang="ts">
import type * as forms from './forms';
import Buttin from '@/components/core/Buttin.vue';
import { watch } from 'vue';

let props = defineProps<{
	ctx: forms.Context,
}>()
let emit = defineEmits(['submit']);

watch(props.ctx.invokeSubmitOnEnter, (newVal) => {
	if (newVal) {
		// reset state
		props.ctx.invokeSubmitOnEnter.value = false;

		// simulate submit click
		onSubmit();
	}
});

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
		<buttin :focal="true" :busy="ctx.busy.value" @click="onSubmit">{{ ctx.submitTitle.value }}</buttin>
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
