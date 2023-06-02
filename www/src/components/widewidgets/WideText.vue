<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { computed } from '@vue/reactivity';

let props = defineProps<{
	label: string,
	modelValue: string | null,
	placeholder?: string,
	password?: boolean,
	focus?: boolean,
}>()
let emit = defineEmits(['update:modelValue']);

let input = ref(null);
let showPassword = ref(false);

let isFakePassword = computed(() => props.password && !showPassword.value);

let inputValue = computed(() => {
	if (isFakePassword.value)
		return "********";
	return props.modelValue ?? "";
});

function showHidePasswords(ev: MouseEvent) {
	showPassword.value = !showPassword.value;
}

function inputStyle(): any {
	if (!props.modelValue) {
		return {
			"border-bottom": "dotted 2px #aaa",
		}
	} else {
		return {
			"border-bottom": "dotted 2px #f5f5f5",
		};
	}
}

function onInput(event: any) {
	// mmkay.. this is crazy. If the form element is a password, then the first time we click on the element,
	// the browser simulates an INPUT event and set the value to the password that it has guessed for this
	// element. We don't want that! We actually KNOW the password, and we just want it shown here. So what
	// we're doing here, is ignoring that first injected INPUT event.
	// Some more context -- this is fired when we click on our "eye" icon to show the password, and this
	// INPUT event is fired BEFORE the click event on the eye.
	//if (!props.ctx.showPasswords.value && props.password) {
	//	console.log("form-text onInput: " + event.target.value + " IGNORED");
	//	return;
	//}
	// OMG... this happens all the time.. the browser just REALLY REALLY wants to mess with this.
	// so annoying.
	// OK.. I'm just going to stop using "password" input boxes, because this is just insane.
	// Long story short -- I just stopped using password input boxes, and fake it by using "*******"
	//console.log("form-text onInput: " + event.target.value);
	emit('update:modelValue', event.target.value);
}

function onFocus() {
	if (props.password) {
		showPassword.value = true;
	}
}

onMounted(() => {
	if (props.focus) {
		(input.value! as any).focus();
	}
})

</script>

<template>
	<div class="widewidget widetext">
		<div class="widelabelTop">
			{{ label }}
		</div>
		<div class="inputContainer">
			<input ref="input" :value="inputValue" @input="onInput($event)" @focus="onFocus" :placeholder="placeholder"
				type="text" autocomplete="off" :style="inputStyle()" />
			<div v-if="password" class="password" @click="showHidePasswords">
				<svg v-if="!showPassword" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="#aaa"
					stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="feather feather-eye">
					<path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"></path>
					<circle cx="12" cy="12" r="3"></circle>
				</svg>
				<svg v-if="showPassword" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="#aaa"
					stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="feather feather-eye-off">
					<path
						d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24">
					</path>
					<line x1="1" y1="1" x2="23" y2="23"></line>
				</svg>
			</div>
		</div>
	</div>
</template>

<style lang="scss" scoped>
@import '@/assets/vars.scss';
@import './widewidget.scss';

.widetext {
	display: flex;
	flex-direction: column;
}

.inputContainer {
	display: flex;
	position: relative;
	width: 100%;
}

input {
	width: 100%;
	font-size: 16px;
	margin-left: $wideLeftMargin;
	box-sizing: border-box;
	border-top: none;
	border-left: none;
	border-right: none;
}

.password {
	position: absolute;
	right: 6px;
	cursor: pointer;
}
</style>