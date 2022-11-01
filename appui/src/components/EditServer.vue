<script setup lang="ts">
import { bestServerName, cloneServer, fetchRegisteredServers, setServerProperty } from '@/nattypes';
import type { Server } from '@/nattypes';
import { onMounted, ref, watch } from 'vue';
import Copy from '@/icons/copy-blue.svg';
import Trash from '@/icons/trash-2.svg';
import SvgButton from '@/components/widgets/SvgButton.vue';
import IconButton from '@/components/widgets/IconButton.vue';
import MsgBox from '@/components/widgets/MsgBox.vue';
import { showToast } from '@/components/widgets/toast';
import { router } from '@/router/routes';
import { globals } from '@/global';
import type { ScannedServer } from '@/scan';

let props = defineProps<{
	publicKey: string,
}>()

let orgServer = ref({} as Server);
let server = ref({} as Server);
let hasChanges = ref(false);
let serverNameCtrl = ref(null);
let showRemove = ref(false);

function shortkey(): string {
	return props.publicKey.substring(0, 12) + "...";
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
		let input = serverNameCtrl.value! as HTMLInputElement;
		if (!input) {
			return;
		}
		server.value.name = input.value;
		//console.log("onNameInput ", server.value.name);
		hasChanges.value = server.value.name !== orgServer.value.name;
	}, 0);
}

function onDelete() {
	showRemove.value = true;
}

function onDeleteConfirm() {
	showRemove.value = false;
	// etc...
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
			<div style="margin-right: 14px">{{ shortkey() }}</div>
			<svg-button :icon="Copy" size="12px" @click="onCopyPublicKey" />
		</div>
		<div class="line">
			<div class="col1">LAN IP Address</div>
			<div style="margin-right: 15px">{{ server.lanIP }}</div>
			<svg-button :icon="Copy" size="12px" @click="onCopyLanIP" />
		</div>
		<div class="line">
			<div class="col1">Name</div>
			<input ref="serverNameCtrl" type="text" :value="server.name" @change="onNameInput" @keydown="onNameInput" />
		</div>
		<div class="bottom">
			<icon-button :class="{ dangerButton: true }" :icon="Trash" @click="onDelete">Delete</icon-button>
			<button :class="{ focalButton: true }" :disabled="!hasChanges" @click="onSave">Save</button>
		</div>
		<msg-box v-if="showRemove" text="Are you sure you want to remove this server?" ok-text="Yes, Delete"
			mode="ok-cancel" :danger="true" @ok="onDeleteConfirm" @cancel="showRemove = false" />
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
	justify-content: space-between;
}
</style>
