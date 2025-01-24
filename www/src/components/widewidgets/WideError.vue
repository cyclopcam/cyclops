<script setup lang="ts">
</script>

<template>
	<div class="wide-error">
		<slot />
	</div>
</template>

<style lang="scss" scoped>
@import '@/assets/vars.scss';

.wide-error {
	color: #d00;
}
</style>