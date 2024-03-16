<script setup lang="ts">
import WideSection from '@/components/widewidgets/WideSection.vue';

let props = defineProps<{
	canSave: boolean,
	status?: string,
	error?: string,
}>()
let emit = defineEmits(['save']);

function statusText(): string {
	// This just looks bad
	//if (props.status === "ok" || props.status === "OK")
	//	return "✔";
	let status = props.status ?? ""
	// meh.. another idea to make the "OK" text disappear after a while, but whatever.
	//if (status.indexOf("[DONE] ") === 0) {
	//}
	return status;
}

</script>

<template>
	<wide-section class="wideSaveCancel">
		<div class="buttons">
			<div v-if="status" class="status">
				{{ statusText() }}
			</div>
			<button class="focalButton" :disabled="!canSave" @click="emit('save')">Save Settings</button>
		</div>
		<div v-if="error" class="error">
			{{ error }}
		</div>
	</wide-section>
</template>

<style lang="scss" scoped>
.wideSaveCancel {
	margin-top: 10px;
}

.buttons {
	display: flex;
	justify-content: flex-end;
	align-items: center;
}

.status {
	margin: 0 14px 0 8px;
	color: #777;
	font-size: 13px;
}

.error {
	color: red;
	margin-top: 10px;
}
</style>
