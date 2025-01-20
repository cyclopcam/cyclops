<script setup lang="ts">
import MobileFullscreen from '@/components/responsive/MobileFullscreen.vue';
import NewUser from '@/components/settings/NewUser.vue';
import { onMounted, ref } from 'vue';
import { globals } from '@/globals';
import { replaceRoute } from "@/router/helpers";
import { useRouter } from 'vue-router';

const router = useRouter();

async function moveToNextStage() {
	await globals.postAuthenticateLoadSystemInfo(false);
	replaceRoute(router, { name: "rtSettingsHome" });
}

onMounted(async () => {
	await globals.waitForPublicKeyLoad();
	await globals.waitForSystemInfoLoad();
})

</script>

<template>
	<mobile-fullscreen>
		<div class="flexColumnCenter">
			<h2 style="text-align: center; margin: 30px 10px">Create a username and password for yourself</h2>
		</div>
		<new-user :is-first-user="true" @finished="moveToNextStage()" />
	</mobile-fullscreen>
</template>
