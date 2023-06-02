<script setup lang="ts">
let props = defineProps<{
	src: string
}>()

// NOT USED 
// NOT USED 
// NOT USED 
// NOT USED 

</script>

<template>
	<img class="panelImage" :src="src" />
</template>

<style lang="scss" scoped>
@import '@/assets/vars.scss';

.panelImage {
	width: 24px;
	height: 24px;
	margin: 0 8px 0 8px;
}
</style>
