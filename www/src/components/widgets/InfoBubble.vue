<script setup lang="ts">
import { ref } from "vue";
import Modal from "../core/Modal.vue";

let props = defineProps<{
	title?: string, // Shows in default popup
	text?: string, // Shows in default popup
	size?: number, // Override icon size
	caption?: string, // If defined, the bubble is "? <caption>"
	tint?: string, // Sent to modal tint
}>()

let isOpen = ref(false);

function textLines(): string[] {
	if (!props.text) {
		return [];
	}
	return props.text.split("\n");
}

function iconStyle(): any {
	if (!props.size) {
		return {};
	}
	return {
		"background-size": props.size + 'px',
		"width": props.size + 'px',
		"height": props.size + 'px',
	}
}

</script>

<template>
	<div>
		<div class="bubbleContainer" @click="isOpen = true">
			<div class="icon background" :style="iconStyle()">
			</div>
			<div v-if="caption" class="captionText">{{caption}}</div>
		</div>
		<modal v-if="isOpen" position="previous" :tint="tint" @close="isOpen = false">
			<div class="bubble">
				<slot />
				<div v-if="title" class="title">{{title}}</div>
				<div v-if="text" class="text">
					<p v-for="line of textLines()" class="text">{{line}}</p>
				</div>
			</div>
		</modal>
	</div>
</template>

<style lang="scss" scoped>
@import '@/assets/vars.scss';

.icon {
	background-image: url('@/icons/help.svg');
	background-size: 16px;
	width: 20px;
	height: 20px;

	@media (max-width: $mobileCutoff) {
		background-size: 20px;
		width: 24px;
		height: 24px;
	}
}

.bubbleContainer {
	background-color: rgb(244, 244, 255);
	border: solid 1px rgb(237, 237, 255);
	border-radius: 5px;
	display: flex;
	align-items: center;
	cursor: pointer;
	padding: 1px 5px 1px 1px;
}

.captionText {
	margin-left: 4px;
	font-size: 12px;
}

.bubble {
	background-color: #fff;
	border-radius: 5px;
	border: solid 1px #555;
	padding: 10px;
	box-shadow: 5px 5px 30px rgba(0, 0, 0, 0.35), 3px 4px 15px rgba(0, 0, 0, 0.45);
}

.title {
	font-size: 14px;
	font-weight: bold;
	padding: 2px;
	margin-bottom: 5px;
}

.text {
	font-size: 14px;
	padding: 5px;
	max-width: 340px;
}

p {
	margin: 0;
	padding: 0;
}
</style>
	