<script setup lang="ts">

async function onSignout() {
	await fetch('/api/auth/logout', { method: 'POST' });
	window.location.href = "/login";
}

</script>

<template>
	<div class="topBar">
		<div></div>
		<div class="signout" @click="onSignout">Sign out</div>
	</div>
</template>

<style scoped lang="scss">
.topBar {
	min-height: 1px;
	display: flex;
	justify-content: space-between;
	background-color: var(--color-topbar-bg);
	color: var(--color-topbar-text);
}

.signout {
	margin: 6px 16px 8px 16px;
}

.signout:hover {
	cursor: pointer;
	text-decoration: underline;
}
</style>
