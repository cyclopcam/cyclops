<script setup lang="ts">
import SvgButton from "./SvgButton.vue";
import Back from '@/icons/back.svg';

</script>
	
<template>
	<div class="fsTopbar">
		<svg-button :icon="Back" size="30px" />
	</div>
</template>
	
<style lang="scss" scoped>
.fsTopbar {
	display: flex;
	padding: 10px 0px 15px 0px;
}
</style>
	