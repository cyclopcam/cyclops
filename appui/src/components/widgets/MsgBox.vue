<script setup lang="ts">
import Modal from '@/components/widgets/Modal.vue';

let props = defineProps<{
	text: string,
	okText?: string, // replace the "OK" text with something custom
	cancelText?: string, // replace the "Cancel" text with something custom
	mode?: string, // "ok", "ok-cancel". default = "ok"
	danger?: boolean, // OK button will have class dangerButton
}>()
let emits = defineEmits(['ok', 'cancel']);

function resolvedMode(): string {
	return props.mode ?? "ok";
}

</script>

<template>
	<modal :tint="danger ? 'mild' : 'none'">
		<div class="msgbox">
			<div class="msg">{{ text }}</div>
			<div class="buttons">
				<button :class="{ dangerButton: danger }" @click="emits('ok')">{{ okText ?? "OK" }}</button>
				<button v-if="resolvedMode() === 'ok-cancel'" @click="emits('cancel')">{{ cancelText ?? "Cancel"
				}}</button>
			</div>
		</div>
	</modal>
</template>

<style lang="scss" scoped>
.msgbox {
	background-color: #fff;
	padding: 30px;
	max-width: 80vw;
	border-radius: 15px;
	box-shadow: 5px 5px 20px rgba(0, 0, 0, 0.3), 1px 1px 5px rgba(0, 0, 0, 0.3);
}

.msg {
	margin-bottom: 30px;
}

.buttons {
	display: flex;
	justify-content: space-between;
}
</style>
