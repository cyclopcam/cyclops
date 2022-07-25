<script setup lang="ts">
import type * as forms from './forms';

let props = defineProps<{
	missing?: boolean,
	ctx: forms.Context,
}>()
let emit = defineEmits(['next']);

function onNext() {
	props.ctx.nextClicked.value = true;
	if (props.ctx.validate())
		emit('next');
}
</script>

<template>
	<div class="flexRowBaseline formBottom" style="align-self: stretch; justify-content: flex-end">
		<div v-if="ctx.showCompleteAllFields" class="required">Complete all required fields</div>
		<button class="focalButton" @click="onNext">Next</button>
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
</style>
