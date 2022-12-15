<script setup lang="ts">
import { bestServerName, natFetchRegisteredServers, natGetLastServer, natGetScreenGrab, natSwitchToRegisteredServer } from '@/nativeOut';
import type { Server } from '@/nativeOut';
import { onMounted, ref } from 'vue';
import SvgButton from '@/components/widgets/SvgButton.vue';
import Edit from '@/icons/edit.svg';
import Plus from '@/icons/plus-circle.svg';
import { pushRoute, router } from '@/router/routes';
import { globals } from '@/global';

// This copy, and globals.waitForLoad is necessary to get consistent reactivity correctness
let servers = ref([] as Server[]);
let root = ref(null);
let canvas = ref(null);

function onConnect(s: Server) {
	natSwitchToRegisteredServer(s.publicKey);
	globals.showExpanded(false);
}

function onEdit(s: Server) {
	pushRoute({ name: 'rtEditServer', params: { publicKey: s.publicKey } });
}

function onAddLocal() {
	pushRoute({ name: 'rtAddLocal' });
}

function onAddRemote() {
	//
}

onMounted(async () => {
	await globals.waitForLoad();
	servers.value = globals.servers;
})

</script>
 
<template>
	<div ref="root" class="pageContainer default">
		<h3>Connections</h3>
		<div class="serverList">
			<div class="server" v-for="s of servers" :key="s.publicKey">
				<a class="link" @click="onConnect(s)" style="font-weight: bold">
					{{ bestServerName(s) }}
				</a>
				<svg-button :icon="Edit" @click="onEdit(s)" />
			</div>
			<div style="height: 5px" />
			<div class="addserver">
				<a class="link addlink" @click="onAddLocal()"><img :src="Plus" class="plus" /> Add Server on local WiFi
					Network</a>
			</div>
			<div class="addserver">
				<a class="link addlink" @click="onAddRemote()"><img :src="Plus" class="plus" /> Add Remote Server</a>
			</div>
		</div>
	</div>
</template>

<style scoped lang="scss">
.default {
	// This is necessary for Vue route left/right (slide-left & slide-right)
	//position: relative;
	//top: 0;

	display: flex;
	flex-direction: column;
}

h3 {
	text-align: left;
}

.server {
	margin: 0px 4px 12px 10px;
	display: flex;
	align-items: center;
	justify-content: space-between;
}

.plus {
	width: 24px;
	height: 24px;
	margin-right: 8px;
}

.addlink {
	display: flex;
	align-items: center;
}

.addserver {
	margin: 24px 4px 12px 10px;
	display: flex;
	align-items: center;
}
</style>
