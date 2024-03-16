<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { computed } from '@vue/reactivity';

let props = defineProps<{
	label: string,
	modelValue: string | null,
	placeholder?: string,
	type?: 'number' | 'text' | 'password',
	focus?: boolean,
	explain?: string,
	unit?: string, // Accompanies units
	units?: string[], // Optional units, like ['GB', 'TB']
}>()
let emit = defineEmits(['update:modelValue', 'blur', 'unit-change']);

let input = ref(null);
let showPassword = ref(false);
let showExplain = ref(false);

let isFakePassword = computed(() => props.type === 'password' && !showPassword.value);

let inputValue = computed(() => {
	if (isFakePassword.value && props.modelValue)
		return "********";
	return props.modelValue ?? "";
});

function showHidePasswords(ev: MouseEvent) {
	showPassword.value = !showPassword.value;
}

function inputType(): string {
	if (props.type === 'number')
		return 'number';

	// If you're specifying units, then you almost definitely are entering a number
	if (props.type == undefined && props.unit)
		return 'number';

	return "text";
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
	if (props.type === 'password') {
		showPassword.value = true;
	}
}

function onBlur() {
	emit('blur');
}

function onUnitClick() {
	let idx = props.units!.indexOf(props.unit!);
	idx = (idx + 1) % props.units!.length;
	emit('unit-change', props.units![idx]);
}

function onExplain() {
	showExplain.value = !showExplain.value;
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
			<div>
				{{ label }}
			</div>
			<div v-if="explain" class="explain" @click="onExplain">
				<svg xmlns="http://www.w3.org/2000/svg" class="icon icon-tabler icon-tabler-help" width="16" height="16"
					viewBox="0 0 24 24" stroke-width="2.0" stroke="#546c85" fill="none" stroke-linecap="round"
					stroke-linejoin="round">
					<path stroke="none" d="M0 0h24v24H0z" fill="none" />
					<path d="M12 12m-9 0a9 9 0 1 0 18 0a9 9 0 1 0 -18 0" />
					<path d="M12 17l0 .01" />
					<path d="M12 13.5a1.5 1.5 0 0 1 1 -1.5a2.6 2.6 0 1 0 -3 -4" />
				</svg>
			</div>
		</div>
		<div class="widelabelExplain" v-if="showExplain">
			{{ explain }}
		</div>
		<div class="inputContainer">
			<input ref="input" :value="inputValue" :type="inputType()" @input="onInput($event)" @focus="onFocus"
				@blur="onBlur" :placeholder="placeholder" autocomplete="off" autocapitalize="off"
				:style="inputStyle()" />
			<div v-if="type === 'password'" class="password" @click="showHidePasswords">
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
			<div v-if="units" class="unit" @click="onUnitClick">
				{{ unit }}
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

.unit {
	margin-left: 10px;
	padding: 2px 4px 2px 4px;
	border: solid 1px #ddd;
	border-radius: 3px;
	background-color: #fdfdfd;
	user-select: none;
}
</style>