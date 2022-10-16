<script setup lang="ts">
import { bestServerName, cloneServer, fetchRegisteredServers, setServerProperty } from '@/nattypes';
import type { Server } from '@/nattypes';
import { onMounted, ref, watch } from 'vue';
import Copy from '@/icons/copy.svg';
import SvgButton from '@/components/widgets/SvgButton.vue';
import { showToast } from '@/components/widgets/toast';
import { router } from '@/router/routes';
import { globals } from '@/global';

let props = defineProps<{
	publicKey: string,
}>()

let orgServer = ref({} as Server);
let server = ref({} as Server);
let hasChanges = ref(false);
let serverNameCtrl = ref(null);

function shortkey(): string {
	return props.publicKey.substring(0, 8) + "...";
}

function onCopyPublicKey() {
	navigator.clipboard.writeText(server.value.publicKey);
	showToast('Public Key copied to clipboard');
}

function onCopyLanIP() {
	navigator.clipboard.writeText(server.value.lanIP);
	showToast('LAN IP copied to clipboard');
}

watch(() => server.value.name, () => {
	onNameInput();
});

function onNameInput() {
	// reasons for timeout and keypress + change events? 
	// mobile is weird!
	// I tried v-model, but it doesn't respond on key changes. It only responds on
	// focus loss of the text input control.
	setTimeout(() => {
		server.value.name = (serverNameCtrl.value! as HTMLInputElement).value;
		//console.log("onNameInput ", server.value.name);
		hasChanges.value = server.value.name !== orgServer.value.name;
	}, 0);
}

function onSave() {
	setServerProperty(server.value.publicKey, "name", server.value.name);
	orgServer.value = cloneServer(server.value);
	router.back();
}

onMounted(async () => {
	await globals.waitForLoad();
	server.value = globals.servers.find(x => x.publicKey === props.publicKey)!;
	if (server.value) {
		orgServer.value = cloneServer(server.value);
	}
})

</script>
 
<template>
	<div class="editServer">
		<h3>Edit Server</h3>
		<div class="line">
			<div class="col1">Public Key</div>
			<div style="margin-right: 10px">{{shortkey()}}</div>
			<svg-button :icon="Copy" size="12px" @click="onCopyPublicKey" />
		</div>
		<div class="line">
			<div class="col1">LAN IP Address</div>
			<div style="margin-right: 10px">{{server.lanIP}}</div>
			<svg-button :icon="Copy" size="12px" @click="onCopyLanIP" />
		</div>
		<div class="line">
			<div class="col1">Name</div>
			<input ref="serverNameCtrl" type="text" :value="server.name" @change="onNameInput" @keydown="onNameInput" />
		</div>
		<div class="bottom">
			<button :class="{focalButton: true}" :disabled="!hasChanges" @click="onSave">Save</button>
		</div>
	</div>
</template>

<style scoped lang="scss">
.editServer {
	//height: 100%;

	// This is necessary for Vue route left/right (slide-left & slide-right)
	//position: relative;
	//top: 0;

	width: 100%;
	box-sizing: border-box;
	display: flex;
	flex-direction: column;
	padding: 0px 20px 20px 20px;
	//border-bottom: solid 4px #000;
	background-color: #fff;
}

.line {
	display: flex;
	align-items: center;
	margin: 10px 4px 4px 8px;
}

.col1 {
	width: 140px;
}

h3 {
	text-align: left;
}

input {
	font-size: 16px;
	width: 160px;
}

.bottom {
	margin: 30px 20px 0 0;
	display: flex;
	justify-content: flex-end;
}
</style>
