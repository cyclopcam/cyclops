<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { computed } from '@vue/reactivity';
import type * as forms from './forms';

let props = defineProps<{
	ctx: forms.Context,
	modelValue: string | null,
	id?: string,
	bigLabel?: string,
	label?: string,
	required?: boolean,
	placeholder?: string,
	password?: boolean,
	focus?: boolean,
	autocomplete?: string,
	submitOnEnter?: boolean,
}>()
let emit = defineEmits(['update:modelValue']);

let input = ref(null);

let isEmpty = computed(() => props.modelValue === null || props.modelValue.trim() === '');

let type = computed(() => {
	if (props.password && !props.ctx.showPasswords.value)
		return "password";
	return "text";
});

function onInput(event: any) {
	emit('update:modelValue', event.target.value);
}

function onKeyPress(ev: KeyboardEvent) {
	//console.log("keypress", ev);
	if (props.submitOnEnter && ev.key === "Enter") {
		props.ctx.invokeSubmitOnEnter.value = true;
		//console.log("wait for it");
	}
}

function showRedDot(): boolean {
	return props.ctx.showRequiredDots && !!props.required && isEmpty.value;
}

function showError(): boolean {
	return !!props.id && !!props.ctx.idToError.value[props.id];
}

function errorMsg(): string {
	return props.id ? (props.ctx.idToError.value[props.id] ?? '') : '';
}

function inputStyle(): any {
	return {
		color: props.ctx.inputColor.value,
		width: '100%',
	};
}

onMounted(() => {
	if (props.focus)
		(input.value! as any).focus();
})

</script>

<template>
	<div class="flexColumn formItem" :style="{ width: props.ctx.inputWidth.value }">
		<slot name="label">
			<div v-if="label" :class="{ label: true, boldLabel: !!bigLabel }">
				{{ label }}
			</div>
		</slot>
		<slot name="bigLabel">
			<div v-if="bigLabel" class="bigLabel">
				{{ bigLabel }}
			</div>
		</slot>
		<div class="flexRowBaseline">
			<div class="flexRowCenter" :style="{ display: 'flex', position: 'relative', width: '100%' }">
				<input ref="input" :value="modelValue" @input="onInput($event)" :placeholder="placeholder" :type="type"
					:autocomplete="autocomplete" :style="inputStyle()" @keypress="onKeyPress" />
				<div v-if="password" class="flexCenter" style="position: absolute; right: 6px; cursor: pointer"
					@click="ctx.showPasswords.value = !ctx.showPasswords.value">
					<svg v-if="!ctx.showPasswords.value" width="20" height="20" viewBox="0 0 24 24" fill="none"
						stroke="#aaa" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"
						class="feather feather-eye">
						<path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"></path>
						<circle cx="12" cy="12" r="3"></circle>
					</svg>
					<svg v-if="ctx.showPasswords.value" width="20" height="20" viewBox="0 0 24 24" fill="none"
						stroke="#aaa" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"
						class="feather feather-eye-off">
						<path
							d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24">
						</path>
						<line x1="1" y1="1" x2="23" y2="23"></line>
					</svg>
				</div>
			</div>
			<div style="width: 25px; text-align: center;">
				<svg v-if="showRedDot()" width="10" height="10">
					<circle cx="5" cy="5" r="3" fill="#d00" />
				</svg>
				<!--
				<svg v-else-if="props.ctx.showRequiredDots && required && !isEmpty" width="10" height="10">
					<circle cx="5" cy="5" r="1.5" fill="#222" />
				</svg>
				-->
			</div>
		</div>
		<div v-if="showError()" class="errorLabel">
			{{ errorMsg() }}
		</div>
	</div>
</template>

<style lang="scss" scoped>
@import '@/assets/vars.scss';
@import '@/components/form/forms.scss';

input {
	border: none;
	border-bottom: $formBorderBottom;
	padding: 2px 2px;
	font-size: $formInputFontSize;
}

input:focus {
	outline: none;
	border-bottom: $formBorderBottomFocus;
}

.errorLabel {
	margin: 12px 8px 0 8px;
	font-size: 14px;
	color: #d00;

}
</style>