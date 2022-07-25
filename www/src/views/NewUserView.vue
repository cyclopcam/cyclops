<script setup lang="ts">
import MobileFullscreen from '@/components/responsive/MobileFullscreen.vue';
import NewUser from '@/components/config/NewUser.vue';

let props = defineProps<{
	isFirstUser: boolean,
}>()

</script>

<template>
	<mobile-fullscreen>
		<div v-if="isFirstUser" class="flexColumnCenter">
			<h2 style="text-align: center; margin: 30px 10px">Create a username and password for yourself</h2>
		</div>
		<new-user :is-first-user="isFirstUser" />
	</mobile-fullscreen>
</template>
