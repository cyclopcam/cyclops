<script setup lang="ts">
import { bestServerName, fetchRegisteredServers, getCurrentServer, getScreenGrab, switchToRegisteredServer } from '@/nattypes';
import type { Server } from '@/nattypes';
import { onMounted, ref } from 'vue';
import SvgButton from '@/components/widgets/SvgButton.vue';
import Edit from '@/icons/edit.svg';
import { router } from '@/router/routes';
import { globals } from '@/global';

// This copy, and globals.waitForLoad is necessary to get consistent reactivity correctness
let servers = ref([] as Server[]);
let root = ref(null);
let canvas = ref(null);

function onConnect(s: Server) {
	switchToRegisteredServer(s.publicKey);
}

function onEdit(s: Server) {
	router.push({ name: 'rtEditServer', params: { publicKey: s.publicKey } });
}

/*
async function updateScreenGrab() {
	let img = await getScreenGrab();
	if (img) {
		let cc = canvas.value! as HTMLCanvasElement;
		let cx = cc.getContext('2d')!;
		cx.putImageData(img, 0, 0);
	}
}

let lastHeight = 0;
function watchForSize() {
	let r = root.value! as HTMLDivElement;
	if (r) {
		let height = r.clientHeight;
		if (height !== lastHeight) {
			lastHeight = height;
			updateScreenGrab();
		}
	}

	setTimeout(watchForSize, 50);
}
*/

onMounted(async () => {
	await globals.waitForLoad();
	servers.value = globals.servers;
	//watchForSize();
})

</script>
 
<template>
	<div ref="root" class="default">
		<h3>Connections</h3>
		<div class="serverList">
			<div class="server" v-for="s of servers" :key="s.publicKey">
				<a class="link" @click="onConnect(s)">
					{{bestServerName(s)}}
				</a>
				<svg-button :icon="Edit" @click="onEdit(s)" />
			</div>
		</div>
	</div>
</template>

<style scoped lang="scss">
.default {
	// This is necessary for Vue route left/right (slide-left & slide-right)
	//position: relative;
	//top: 0;

	width: 100%;
	//height: 300px;
	background-color: #fff;
	box-sizing: border-box;
	display: flex;
	flex-direction: column;
	padding: 0px 20px 20px 20px;
	//border-bottom: solid 4px #000;
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
</style>
