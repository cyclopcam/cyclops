<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { computed } from '@vue/reactivity';
import type * as forms from './forms';

let props = defineProps<{
	ctx: forms.Context,
	label?: string,
	modelValue: string,
	required?: boolean,
	placeholder?: string,
	password?: boolean,
	focus?: boolean,
}>()
let emit = defineEmits(['update:modelValue']);

let showPassword = ref(false);
let input = ref(null);

let isEmpty = computed(() => props.modelValue.trim() === '');

let type = computed(() => {
	if (props.password && !showPassword.value)
		return "password";
	return "text";
});

function onInput(event: any) {
	emit('update:modelValue', event.target.value);
}

onMounted(() => {
	if (props.focus)
		(input.value! as any).focus();
})

</script>

<template>
	<div class="flexColumn formText">
		<slot name="label">
			<div v-if="label" class="label">
				{{ label }}
			</div>
		</slot>
		<div class="flexRowBaseline">
			<div class="flexRowCenter"
				:style="{ display: 'flex', position: 'relative', width: props.ctx.inputWidth.value }">
				<input ref="input" :value="modelValue" @input="onInput($event)" :placeholder="placeholder" :type="type"
					style="width: 100%" />
				<div v-if="password" class="flexCenter" style="position: absolute; right: 6px;"
					@click="showPassword = !showPassword">
					<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="#777" stroke-width="2"
						stroke-linecap="round" stroke-linejoin="round" class="feather feather-eye">
						<path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"></path>
						<circle cx="12" cy="12" r="3"></circle>
					</svg>
				</div>
			</div>
			<div style="width: 25px; text-align: center;">
				<svg v-if="props.ctx.showRequiredDots && required && isEmpty" width="10" height="10">
					<circle cx="5" cy="5" r="3" fill="#d00" />
				</svg>
				<!--
				<svg v-else-if="props.ctx.showRequiredDots && required && !isEmpty" width="10" height="10">
					<circle cx="5" cy="5" r="1.5" fill="#222" />
				</svg>
				-->
			</div>
		</div>
	</div>
</template>

<style lang="scss" scoped>
.formText {
	margin: 14px 0px;
}

.label {
	margin: 2px 0px 3px 0px;
	font-size: 13.5px;
	color: #888;
}

input {
	//border: solid 1px #ccc;
	border: none;
	border-bottom: solid 1px #ddd;
	//border-radius: 3px;
	padding: 1px 1px;
	font-size: 16px;
}

input:focus {
	outline: none;
	border-bottom: solid 1px #888;
}

//input:focus-visible {
//	border: solid 1px #000;
//}
::placeholder {
	color: #bbb;
}
</style>