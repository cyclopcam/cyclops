// MobileFullscreen is a container for a page that would fill the screen on mobile,
// and must work well there. But if it's accessed on desktop, then we still want
// it to look reasonable. So on desktop we center it, and give it a limited width.
<script setup lang="ts">
</script>

<template>
	<div class="mobileFullscreenOuter">
		<div class="mobileFullscreenInner">
			<slot></slot>
		</div>
	</div>
</template>

<style lang="scss" scoped>
@import '@/assets/vars.scss';

.mobileFullscreenOuter {
	@media only screen and (min-width: $mobileCutoff) {
		display: flex;
		flex-direction: column;
		align-items: center;
	}
}

.mobileFullscreenInner {
	@media only screen and (min-width: $mobileCutoff) {
		max-width: 800px;
	}

	margin: 20px 15px;
}
</style>