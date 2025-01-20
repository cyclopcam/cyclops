<script setup lang="ts">
import back from "@/icons/chevron-left.svg";
import SvgButton from "@/components/widgets/SvgButton.vue";
import { popRoute } from "@/router/helpers";
import { useRouter } from "vue-router";

const router = useRouter();

function onBack() {
	popRoute(router);
}

</script>

<template>
	<div class="wide-page-heading">
		<div class="flex" style="width: 60px">
			<svg-button :icon="back" size="28px" style="margin-left:10px" @click="onBack" />
		</div>
		<slot />
		<div style="width: 40px" />
	</div>
</template>

<style lang="scss" scoped>
@import '@/assets/vars.scss';
@import './widewidget.scss';

.wide-page-heading {
	width: 100%;
	box-sizing: border-box;
	padding: 14px 16px 14px 0px;
	margin-bottom: 8px;
	background-color: #fff;
	box-shadow: 0px 1px 3px rgba(0, 0, 0, 0.08);
	//font-weight: 600;
	display: flex;
	align-items: center;
	justify-content: space-between;
	border-bottom: 1px solid #bbb;
}
</style>