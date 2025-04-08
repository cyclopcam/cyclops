<script setup lang="ts">
let props = defineProps<{
	text: string
}>()

function cleanText(): string {
	if (props.text.startsWith("ERROR:")) {
		return props.text.substring(6);
	}
	return props.text;
}

function isError(): boolean {
	return props.text.startsWith("ERROR:");
}

</script>

<template>
	<p :class="{ error: isError() }">
		{{ cleanText() }}
	</p>
</template>

<style lang="scss" scoped>
.error {
	color: #d00;
}
</style>
