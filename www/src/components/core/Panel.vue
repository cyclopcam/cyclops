<script setup lang="ts">
</script>

<template>
	<div class="panel">
		<slot />
	</div>
</template>

<style lang="scss" scoped>
@import '@/assets/vars.scss';

.panel {
	display: flex;
	flex-direction: column;
	align-items: stretch;
	//background-color: beige;
	//gap: 10px;

	width: 300px;
	//border-left: solid 1px #ddd;
	//border-right: solid 1px #ddd;

	@media (max-width: $mobileCutoff) {
		//border: none;
		width: 100vw;
	}

}
</style>
