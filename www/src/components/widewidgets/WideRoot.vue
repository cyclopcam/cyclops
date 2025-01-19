<script setup lang="ts">
import WidePageHeading from '@/components/widewidgets/WidePageHeading.vue';

let props = defineProps<{
	title?: string,
}>();

</script>

<template>
	<div class="wideRoot">
		<wide-page-heading v-if="title !== undefined">{{ title }}</wide-page-heading>
		<slot />
	</div>
</template>

<style lang="scss" scoped>
@import '@/assets/vars.scss';
@import './widewidget.scss';
</style>