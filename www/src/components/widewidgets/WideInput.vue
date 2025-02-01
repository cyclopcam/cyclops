<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { computed } from '@vue/reactivity';
import WidePopupText from '@/components/widewidgets/WidePopupText.vue';
import WidePopupOptions from '@/components/widewidgets/WidePopupOptions.vue';
import WidePopupExplain from '@/components/widewidgets/WidePopupExplain.vue';
import Toggle from '@/components/widgets/Toggle.vue';

enum Type {
	Number = 'number',
	Text = 'text',
	Password = 'password',
	Options = 'options',
	Boolean = 'boolean',
}

let props = defineProps<{
	label: string,
	modelValue: string | boolean | null,
	options?: any[], // Can be an array of strings, or array of objects of type {value:string, label: string}
	placeholder?: string,
	type?: 'number' | 'text' | 'password' | 'options' | 'boolean',
	okText?: string, // Default is "Save", but you might want to use "OK" if pressing this button doesn't actually save to server
	focus?: boolean,
	explain?: string,
	unit?: string, // Accompanies units
	units?: string[], // Optional units, like ['GB', 'TB']
	required?: boolean, // Default false
}>()
let emit = defineEmits(['update:modelValue', 'change', 'unit-change']);

let input = ref(null);
let showPassword = ref(false);
let showExplain = ref(false);
let showPopupText = ref(false);
let showPopupOptions = ref(false);
let showUnitPicker = ref(false);

let isFakePassword = computed(() => props.type === 'password' && !showPassword.value);

let inputValue = computed(() => {
	if (isFakePassword.value && props.modelValue)
		return "********";
	return props.modelValue ?? "";
});

function getValueText(): string {
	if (getType() === Type.Options)
		return getOptionLabelFromValue(props.modelValue as string);
	return inputValue.value as string;
}

function isEmpty(): boolean {
	return props.modelValue == null || props.modelValue === "";
}

function getType(): Type {
	if (props.type)
		return props.type as Type;
	if (props.options)
		return Type.Options;
	return Type.Text;
}

function textInputHTMLType(): 'number' | 'text' {
	if (props.type === 'number')
		return 'number';
	return 'text';
}

function getOptionValue(opt: any) {
	return typeof opt === 'object' ? opt.value : opt;
}

// Given a value, such as "motion", return the label associated with it, such as "Whenever there is motion"
function getOptionLabelFromValue(value: string | null) {
	if (value == null || props.options === undefined)
		return "";
	if (typeof props.options[0] === 'object') {
		let obj = props.options.find(o => o.value === value);
		if (obj)
			return obj.label;
		return value;
	}
	return value;
}

function onUnitClick() {
	showUnitPicker.value = true;
	//let idx = props.units!.indexOf(props.unit!);
	//idx = (idx + 1) % props.units!.length;
	//emit('unit-change', props.units![idx]);
}

function onExplain() {
	showExplain.value = !showExplain.value;
}

function onValueClick() {
	if (getType() === Type.Options)
		showPopupOptions.value = true;
	else
		showPopupText.value = true;
}

function onTextEdit(value: any) {
	showPopupText.value = false;
	emit('update:modelValue', value);
	emit('change', value);
}

function onSelectOption(opt: any) {
	showPopupOptions.value = false;
	emit('update:modelValue', getOptionValue(opt));
	emit('change', getOptionValue(opt));
}

function onToggleChange(value: boolean) {
	//console.log("WideInput onToggleChange", value);
	emit('update:modelValue', value);
	emit('change', value);
}

function onSelectUnit(opt: any) {
	showUnitPicker.value = false;
	emit('unit-change', opt);
}

onMounted(() => {
	if (props.focus) {
		(input.value! as any).focus();
	}
})

</script>

<template>
	<div class="wideinput wide-section-element">
		<div class="wideinput-label">
			<div>
				{{ label }}
			</div>
			<div v-if="explain" class="explain" @click="onExplain">
				<svg xmlns="http://www.w3.org/2000/svg" class="icon icon-tabler icon-tabler-help" width="16" height="16"
					viewBox="0 0 24 24" stroke-width="2.0" stroke="#94a4b5" fill="none" stroke-linecap="round"
					stroke-linejoin="round">
					<path stroke="none" d="M0 0h24v24H0z" fill="none" />
					<path d="M12 12m-9 0a9 9 0 1 0 18 0a9 9 0 1 0 -18 0" />
					<path d="M12 17l0 .01" />
					<path d="M12 13.5a1.5 1.5 0 0 1 1 -1.5a2.6 2.6 0 1 0 -3 -4" />
				</svg>
			</div>
		</div>
		<div class="wideinput-value-container">
			<toggle v-if="type === 'boolean'" v-model="modelValue as boolean" @change="onToggleChange" />
			<div v-else class="wideinput-value" :class="{ empty: isEmpty() && required }" @click="onValueClick">{{
				getValueText() }}</div>
			<div v-if="units" class="unit" @click="onUnitClick">
				{{ unit }}
			</div>
		</div>
		<wide-popup-text v-if="showPopupText" :title="label" :type="textInputHTMLType()" :okText="okText ?? 'Save'"
			:value="(modelValue as string | null) ?? ''" @cancel="showPopupText = false" @ok="onTextEdit" />
		<wide-popup-options v-if="showPopupOptions" :title="label" :options="options!" :value="inputValue as string"
			@cancel="showPopupOptions = false" @select="onSelectOption" />
		<wide-popup-explain v-if="showExplain" @close="showExplain = false" :text="explain" />
		<wide-popup-options v-if="showUnitPicker" title="Units" :options="units!" :value="unit!"
			@cancel="showUnitPicker = false" @select="onSelectUnit" />
	</div>
</template>

<style lang="scss" scoped>
@import '@/assets/vars.scss';
@import './widewidget.scss';

.wideinput {
	width: 100%;
	box-sizing: border-box;
	display: flex;
	align-items: center;
	justify-content: space-between;
	font-size: 15px;
	min-height: 54px;
}

.wideinput-label {
	display: flex;
	margin-left: 0px;
	font-weight: 500;
	color: #000;
	user-select: none;
}

.explain {
	margin-left: 8px;
}

.wideinput-value-container {
	display: flex;
	align-items: center;
	max-width: 45%;
	margin-right: 2px;
	color: #666;
}

.wideinput-value {
	white-space: nowrap;
	overflow: hidden;
	text-overflow: ellipsis;
	border-bottom: solid 1.5px rgba(0, 0, 0, 0);
}

.empty {
	min-height: 26px;
	min-width: 80px;
	border-bottom: dashed 1.5px #cf4b12;
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