<script setup lang="ts">
import { RouterView } from 'vue-router'
</script>

<template>
	<router-view />
</template>

<!-- NOTE! These styles are not scoped, which is why we import 'base' -->
<style lang="scss">
#app {
	height: 100%;
	//background-color: #efe;
}

@import '@/assets/base.scss';
</style>
