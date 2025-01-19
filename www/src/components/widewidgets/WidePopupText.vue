<script setup lang="ts">
import Modal from '@/components/widgets/Modal.vue';
import { nextTick, onMounted, ref } from 'vue';
import { computed } from '@vue/reactivity';

let props = defineProps<{
	title: string,
	value: string | null,
	okText: string,
	placeholder?: string,
	type?: 'number' | 'text' | 'password',
	unit?: string, // Accompanies units
	units?: string[], // Optional units, like ['GB', 'TB']
}>()

let emit = defineEmits(['cancel', 'ok', 'unit-change']);

let input = ref(null);
let showPassword = ref(false);
let currentValue = ref(props.value);

let isFakePassword = computed(() => props.type === 'password' && !showPassword.value);

// NOTE: See this rant for why we don't use a real "password" input type.
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


let inputValue = computed(() => {
	if (isFakePassword.value && currentValue.value)
		return "********";
	return currentValue.value ?? "";
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

function onInput(event: any) {
	currentValue.value = event.target.value;
}

function onUnitClick() {
	let idx = props.units!.indexOf(props.unit!);
	idx = (idx + 1) % props.units!.length;
	emit('unit-change', props.units![idx]);
}

function onFocus() {
	if (props.type === 'password') {
		showPassword.value = true;
	}
}

function onOK() {
	emit('ok', currentValue.value);
}

onMounted(async () => {
	await nextTick();
	(input.value! as any).focus();
})

</script>

<template>
	<modal @close="emit('cancel')" tint="extradark" :scrollable="true" :fullscreen="true">
		<div class="container">
			<div class="flexCenter title">{{ title }}</div>

			<div class="inputContainer">
				<input ref="input" :value="inputValue" :type="inputType()" @input="onInput($event)" @focus="onFocus"
					:placeholder="placeholder" autocomplete="off" autocapitalize="off" />
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

			<div class="bottom">
				<div class="flexCenter bottomButton cancel" @click="emit('cancel')">Cancel</div>
				<div class="bottomDivider" />
				<div class="flexCenter bottomButton ok" @click="onOK">{{ okText }}</div>
			</div>
		</div>
	</modal>
</template>

<style lang="scss" scoped>
.container {
	background: #fff;
	width: 90vw;
	border-radius: 15px;
}

.title {
	font-weight: bold;
	padding: 24px 24px 0px 24px;
}

.inputContainer {
	display: flex;
	position: relative;
	width: 100%;
}

input {
	width: 100%;
	font-size: 15px;
	margin: 24px 12px 30px 12px;
	box-sizing: border-box;
	padding: 8px 8px;
	border: none;
}

input:focus {
	outline: solid 1px rgba(219, 166, 59, 0.1);
	border-radius: 3px;
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


.bottom {
	display: flex;
	justify-content: space-between;
	height: 45px;
	width: 100%;
	border-top: solid 1px #ddd;
}

.bottomButton {
	flex: 1 1 auto;
	width: 1px;
	height: 100%;
	user-select: none;
	cursor: pointer;
}

.bottomDivider {
	width: 1px;
	background-color: #ddd;
}

.ok {
	font-weight: bold;
}
</style>
