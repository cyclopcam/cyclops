<script setup lang="ts">
import { OntologyLevel, type OntologyTag } from '@/recording/recording';
import InfoBubble from '../../widgets/InfoBubble.vue';

let props = defineProps<{
	tag: OntologyTag,
	selectable?: boolean,
	selected?: boolean,
	showInfoBubble?: boolean,
}>();
let emits = defineEmits(['select']);

function isAlarm() {
	return props.tag.level === OntologyLevel.Alarm;
}

function isRecord() {
	return props.tag.level === OntologyLevel.Record;
}

function isIgnore() {
	return props.tag.level === OntologyLevel.Ignore;
}

function infoTitle() {
	switch (props.tag.level) {
		case OntologyLevel.Alarm:
			return "Alarm";
		case OntologyLevel.Record:
			return "Record";
		case OntologyLevel.Ignore:
			return "Ignore";
	}
}

function infoText() {
	switch (props.tag.level) {
		case OntologyLevel.Alarm:
			return "If the system is armed and this action is seen, the alarm will be raised.\n" +
				"If the system is not armed, then this event will be recorded.";
		case OntologyLevel.Record:
			return "Whenever this action is seen, it will be recorded.\n" +
				"The alarm will not be raised.";
		case OntologyLevel.Ignore:
			return "The system will ignore this completely.";
	}
}

</script>
	
<template>
	<div class="flex">
		<label class="flex">
			<input v-if="selectable" type="checkbox" :checked="selected" @input="$emit('select')" />
			<div :class="{name: true, alarmBG: isAlarm(), recordBG: isRecord(), ignoreBG: isIgnore()}">{{tag.name}}
			</div>
		</label>
		<info-bubble v-if="showInfoBubble" style="margin-left: 10px" :title="infoTitle()" :text="infoText()" />
	</div>
</template>
	
<style lang="scss" scoped>
@import './tag.scss';

input {
	width: 18px;
	height: 18px;
}

.name {
	cursor: pointer;
	font-size: 14px;
	width: 80px;
	padding: 3px 6px;
	border-radius: 5px;
	user-select: none;
	border-width: 1px;
	border-style: solid;
	margin-left: 5px;
}
</style>
	