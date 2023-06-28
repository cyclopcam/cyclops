<script setup lang="ts">
import Modal from '@/components/widgets/Modal.vue';

let props = defineProps<{
	msg: string,
	yesText: string,
	danger?: boolean,
	noText?: string,
}>();

let emits = defineEmits(['ok', 'cancel']);

</script>

<template>
	<modal tint="dark">
		<div class="root">
			<div class="msg">{{ msg }}</div>
			<div class="buttons">
				<button @click="emits('ok')" :class="{ dangerButton: danger }">{{ yesText }}</button>
				<button @click="emits('cancel')" class="cancel">{{ noText ?? "Cancel" }}</button>
			</div>
		</div>
	</modal>
</template>

<style lang="scss" scoped>
.root {
	background: #fff;
	width: 90vw;
	border-radius: 5px;
	display: flex;
	flex-direction: column;
	align-items: center;
}

.msg {
	margin: 40px 30px 40px 30px;
	font-size: 18px;
}

.buttons {
	display: flex;
	margin-bottom: 20px;
}

.cancel {
	margin-left: 16px;
}
</style>
