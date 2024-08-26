<script setup lang="ts">
import WideSection from '@/components/widewidgets/WideSection.vue';
import WideText from '@/components/widewidgets/WideText.vue';
import WideSaveCancel from '@/components/widewidgets/WideSaveCancel.vue';
import WideDropdown from '@/components/widewidgets/WideDropdown.vue';
import { ref, watch, onMounted } from 'vue';
import { byteSizeUnit, formatByteSize, kibiSplit, type ByteSizeUnit } from '@/util/kibi';
import { fetchOrErr } from '@/util/util';
import { globals } from '@/globals';

let props = defineProps<{
}>()

type RecordingMode = 'always' | 'movement' | 'detection';

// SYNC-SYSTEM-CONFIG-JSON
interface ConfigJSON {
	recording: RecordingJSON;
	tempFilePath: string;
	arcServer: string;
	arcApiKey: string;
}

// SYNC-SYSTEM-RECORDING-CONFIG-JSON
interface RecordingJSON {
	mode?: RecordingMode;
	path?: string;
	maxStorageSize?: string;
}

let config = ref(null as ConfigJSON | null);
let archiveDir = ref(''); // the root of the archive
let maxStorage = ref(''); // max storage space
let spaceAtArchive = ref(0); // measured by server
let spaceAtArchiveUsed = ref(0); // measured by server
let spaceAtArchiveBusy = ref(false);
let spaceAtArchiveError = ref('');
let storageUnit = ref('GB');
let recordingMode = ref('always' as RecordingMode);
let saveError = ref('');
let saveStatus = ref('');

let archiveDirExplain = 'This is where video recordings are stored.';
let maxStorageExplain = 'The maximum amount of space to use for video recordings.';
let storageUnits = ["KB", "MB", "GB", "TB", "PB"];
let recordingModes = [
	{ value: 'always', label: 'Always' },
	{ value: 'movement', label: 'When movement is detected' },
	{ value: 'detection', label: 'When an object is detected' },
];

//watch(archiveDir, async (newVal) => {
//});

async function onArchiveBlur() {
	await measureSpaceAvailable();
}

async function measureSpaceAvailable() {
	if (archiveDir.value.length <= 1)
		return;

	spaceAtArchiveBusy.value = true;
	let r = await fetchOrErr('/api/config/measureStorageSpace?path=' + encodeURIComponent(archiveDir.value));
	spaceAtArchiveBusy.value = false;
	if (r.ok) {
		// response has "available" and "used". We want "available" + "used", because all
		// of the space inside 'path' is available for us.
		let rj = await r.r.json();
		spaceAtArchive.value = rj.available + rj.used;
		spaceAtArchiveUsed.value = rj.used;
		spaceAtArchiveError.value = '';
	} else {
		spaceAtArchiveError.value = r.error;
	}
}

function byteSizeUnitForSpace(): ByteSizeUnit {
	if (spaceAtArchiveBusy.value) {
		return 'bytes';
	}
	return byteSizeUnit(spaceAtArchive.value);
}

function spaceUsed(): string {
	if (spaceAtArchiveBusy.value)
		return "busy...";
	return formatByteSize(spaceAtArchiveUsed.value, byteSizeUnitForSpace(), false);
}

function spaceAvailable(): string {
	if (spaceAtArchiveBusy.value)
		return "";
	return formatByteSize(spaceAtArchive.value, byteSizeUnitForSpace());
}

function onStorageUnitChange(unit: string) {
	storageUnit.value = unit;
}

function canSave(): boolean {
	return config.value !== null && JSON.stringify(makeAlteredConfig()) !== JSON.stringify(config.value);
}

function makeAlteredConfig(): ConfigJSON | null {
	if (config.value === null)
		return null;
	let cfg = JSON.parse(JSON.stringify(config.value)) as ConfigJSON;
	cfg.recording.path = archiveDir.value;
	cfg.recording.maxStorageSize = maxStorage.value + ' ' + storageUnit.value;
	cfg.recording.mode = recordingMode.value;
	return cfg;
}

async function onSave() {
	saveStatus.value = "Saving...";
	let altered = makeAlteredConfig();
	let r = await fetchOrErr('/api/config/settings', {
		method: 'POST',
		headers: {
			'Content-Type': 'application/json',
		},
		body: JSON.stringify(altered),
	});
	if (r.ok) {
		let rj = await r.r.json();
		config.value = altered;
		console.log("Saved");
		saveError.value = "";
		if (rj.needsRestart) {
			saveStatus.value = "Restarting...";
			let restart = await globals.restart(10);
			if (restart === "") {
				// success
				saveStatus.value = "Settings Applied";
			} else {
				saveError.value = restart;
			}
		} else {
			saveStatus.value = "Settings Applied";
		}
	} else {
		saveStatus.value = "";
		saveError.value = r.error;
		console.error("Failed to save: " + r.error);
	}

}

async function loadConfig() {
	let r = await fetchOrErr('/api/config/settings');
	if (r.ok) {
		config.value = await r.r.json() as ConfigJSON;
		archiveDir.value = config.value.recording.path ?? '';
		recordingMode.value = config.value.recording.mode ?? 'always';
		if (config.value.recording.maxStorageSize === undefined) {
			maxStorage.value = '0';
			storageUnit.value = 'GB';
		} else {
			let { value, unit } = kibiSplit(config.value.recording.maxStorageSize ?? '');
			maxStorage.value = value.toString();
			storageUnit.value = unit;
		}
	} else {
		console.error("Failed to load config: " + r.error);
	}

	//console.log("v1", JSON.stringify(makeAlteredConfig()));
	//console.log("v2", JSON.stringify(config.value));
}

onMounted(async () => {
	await loadConfig();
	await measureSpaceAvailable();
});

</script>

<template>
	<div class="wideRoot">
		<wide-text label="Video Location" v-model="archiveDir" :explain="archiveDirExplain" @blur="onArchiveBlur" />
		<wide-section>
			<div v-if="spaceAtArchiveError" class="spaceAvailable error">
				{{ spaceAtArchiveError }}
			</div>
			<div v-else class="spaceAvailable">
				<span class="spaceAvailableMute">Space used</span> {{ spaceUsed() }} / {{ spaceAvailable() }}
			</div>
		</wide-section>
		<wide-text label="Max storage space" v-model="maxStorage" :explain="maxStorageExplain" :unit="storageUnit"
			:units="storageUnits" @unit-change="onStorageUnitChange" />
		<!-- <wide-text label="Fake password" v-model="fakePassword" type="password" />  -->
		<wide-dropdown label="When to record" v-model="recordingMode" :options="recordingModes" />
		<wide-save-cancel :can-save="canSave()" :error="saveError" :status="saveStatus" @save="onSave" />
	</div>
</template>

<style lang="scss" scoped>
.spaceAvailable {
	display: flex;
	justify-content: flex-end;
	margin: 4px 0;
	color: #444;
}

.spaceAvailableMute {
	color: #777;
	margin-right: 12px;
}


.error {
	color: red;
}
</style>
