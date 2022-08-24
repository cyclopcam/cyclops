<script setup lang="ts">
import Modal from './Modal.vue';

// Our name "Menue" is stupid, but we need it to avoid conflict with the built-in HTML <menu> element

interface MenuItem {
	// You probably also want an "action: string," to match against
	title: string,
}

let props = defineProps<{
	items: MenuItem[]
}>()
let emits = defineEmits(['close', 'select']);

function onSelect(item: MenuItem) {
	emits('close');
	emits('select', item);
}

</script>

<template>
	<modal position="previous" relative="under" @close="$emit('close')">
		<div class="container">
			<div v-for="item of items" class="item" @click="onSelect(item)">
				{{ item.title }}
			</div>
		</div>
	</modal>
</template>

<style lang="scss" scoped>
@import '@/assets/vars.scss';

.container {
	background-color: #fff;
	box-shadow: 3px 3px 11px rgba(0, 0, 0, 0.2), 3px 3px 7px rgba(0, 0, 0, 0.2);
	border-radius: 3px;
	user-select: none;
	font-size: 15px;
}

.item {
	padding: 6px 10px;
	border-radius: 3px;
	cursor: pointer;

	@media (max-width: $mobileCutoff) {
		padding: 12px 16px;
	}
}

.item:hover {
	background-color: #ddd;
}

.item:first-child {
	padding-top: 6px;

	@media (max-width: $mobileCutoff) {
		padding-top: 14px;
	}
}

.item:last-child {
	padding-bottom: 6px;

	@media (max-width: $mobileCutoff) {
		padding-bottom: 14px;
	}
}
</style>
