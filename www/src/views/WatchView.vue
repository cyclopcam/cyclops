<script setup lang="ts">
import { globals } from '@/globals';
import Player from '@/components/camera/Player.vue';
</script>

<template>
	<Player v-for="camera of globals.cameraInfos" :camera='camera' />
</template>
