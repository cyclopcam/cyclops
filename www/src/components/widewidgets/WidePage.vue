<script setup lang="ts">
// A generic top-level component that will host a wide-root.
// Sigh.. this feels too complicated
// NOT USED!!!! 
</script>

<template>
	<div class="widePage">
		<slot />
	</div>
</template>

<style lang="scss" scoped>
@import '@/assets/vars.scss';

.widePage {
	height: 100%;
	display: flex;
	flex-direction: column;
	z-index: 0;
}
</style>
