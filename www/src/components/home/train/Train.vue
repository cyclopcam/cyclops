<script setup lang="ts">
import { pushRoute } from "@/router/helpers";
import { onMounted } from 'vue';
import { useRouter } from 'vue-router';

const router = useRouter();

function onRecordNew() {
	pushRoute(router, { name: "rtTrainRecord" });
}

onMounted(() => {
	//console.log(router.currentRoute.value);
})
</script>

<template>
	<div class="flexColumnCenter">
		<router-view @recordNew="onRecordNew" v-slot="{ Component, route }">
			<transition :name="route.meta.transitionName">
				<component :is="Component" />
			</transition>
		</router-view>
	</div>
</template>

<style lang="scss" scoped>
//.train {
//	//background-color: cornsilk;
//	//margin: 10px 10px 10px 10px;
//}
</style>
