<script setup lang="ts">
import { CameraRecord } from '@/db/config/configdb';
import { onMounted, ref } from 'vue';
import NewCameraConfig from '@/components/settings/NewCameraConfig.vue';
import { encodeQuery, fetchOrErr } from '@/util/util';
import Error from '../core/Error.vue';
import Buttin from '../core/Buttin.vue';
import ScannedCamera from './ScannedCamera.vue';
import ConfigureCameraButton from './ConfigureCameraButton.vue';
import Separator from '../core/separator.vue';
import PanelButton from "../core/PanelButton.vue";
import Panel from "../core/Panel.vue";
import video from "@/icons/video.svg";
import addIcon from "@/icons/plus-circle.svg";
import { router } from '@/router/routes';

let props = defineProps({
	isInitialSetup: {
		type: Boolean,
		default: false,
	}
});

let emits = defineEmits(['finished']);

let error = ref('');
let configured = ref([] as CameraRecord[]); // cameras in the server DB
let scanned = ref([] as CameraRecord[]); // scanned on the LAN, and NOT in the DB
let busyScanning = ref(false);
let numTotalScans = ref(0);
let numExplicitScans = 0;
let expanded = ref(null as CameraRecord | null);
let sureContinue = ref("");

onMounted(async () => {
	fetchExisting();
	if (props.isInitialSetup) {
		scan(false);
	}
})

async function onScanAgain() {
	scan(true);
}

async function scan(isExplicit: boolean) {
	numTotalScans.value++;
	if (isExplicit) {
		numExplicitScans++;
	}
	// raise the timeout with each scan, in case there are slow cameras
	let timeoutMS = 50 * Math.pow(2, Math.min(numExplicitScans, 3));
	let options = {
		"cache": isExplicit ? "nocache" : "",
		"timeout": timeoutMS
	};
	scanned.value = [];
	busyScanning.value = true;
	let r = await fetchOrErr('/api/config/scanNetworkForCameras?' + encodeQuery(options), { method: 'POST' });
	if (r.ok) {
		scanned.value = ((await r.r.json()) as []).map(x => CameraRecord.fromJSON(x));
	} else {
		error.value = r.error;
	}
	busyScanning.value = false;
}

async function fetchExisting() {
	let r = await fetchOrErr('/api/config/cameras');
	if (r.ok) {
		configured.value = ((await r.r.json()) as []).map(x => CameraRecord.fromJSON(x));
	}
}

function onAddCamera(cam: CameraRecord) {
	configured.value.push(cam);
	expanded.value = null;
}

function onContinue() {
	if (sureContinue.value === '' && configured.value.length === 0) {
		sureContinue.value = "You haven't added any cameras yet. Are you sure you want to continue?"
		return;
	}
	emits('finished');
}

function isConfigured(cam: CameraRecord): boolean {
	for (let c of configured.value) {
		if (c.host === cam.host) {
			return true;
		}
	}
	return false;
}

// TODO: Figure out how to detect if we have a child route, and also how to react to route changes.
// Only show <router-view /> if we have a child route, and if we do have a child route, don't
// show the rest of us.
// Actually wait.. all the rest of us is simply the legacy Add Camera initial setup stuff, so I
// guess we're either showing a panel, or a child.
onMounted(() => {
});

</script>

<template>
	<div>
		<router-view />
		<error v-if="error">{{ error }}</error>
		<div class="existing">
			<panel>
				<panel-button v-for="camera of configured" :icon="video" icon-size="18px"
					route-target="rtSettingsCameras">{{ camera.name }}</panel-button>
				<panel-button route-target="rtSettingsCamerasAdd" :icon="addIcon" icon-size="18px" :icon-tweak-x="-1">Add
					Camera</panel-button>
			</panel>
		</div>
		<div v-if="numTotalScans !== 0" class="scanned">
			<p style="margin-bottom: 30px">The following devices were found on your network</p>
			<div v-for="camera of scanned">
				<div v-if="expanded === camera" class="expanded shadow9">
					<new-camera-config :camera="camera" @add="onAddCamera(camera)" />
				</div>
				<scanned-camera v-if="expanded !== camera" :camera="camera" :is-configured="isConfigured(camera)"
					@add="expanded = camera" style="margin: 10px 0px" />
			</div>
			<div class="flex" style="justify-content: flex-end; margin: 25px 0px;">
				<buttin :busy="busyScanning" @click="onScanAgain">Scan Again</buttin>
			</div>
		</div>
		<div v-else>
			<div class="flex" style="justify-content: flex-end; margin: 25px 0px;">
				<buttin :busy="busyScanning" @click="onScanAgain">Scan Local Network</buttin>
			</div>
		</div>
		<div v-if="isInitialSetup">
			<separator />
			<div class="flex" style="justify-content: flex-end; align-items: center">
				<div v-if="sureContinue" style="color: #d80; max-width: 260px; margin-right: 20px">{{ sureContinue }}</div>
				<button @click="onContinue">{{ sureContinue ? "Yes, Continue" : "Continue" }}</button>
			</div>
		</div>
	</div>
</template>

<style lang="scss" scoped>
.expanded {
	padding: 15px 10px 10px 20px;
	border-radius: 5px;
}
</style>
