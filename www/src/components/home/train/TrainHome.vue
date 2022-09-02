<script setup lang="ts">
import { globals } from "@/globals";
import router from '@/router/routes';
import { onMounted, ref } from 'vue';
import PanelButton from '../../core/PanelButton.vue';
import RedDot from "@/icons/red-dot.svg";
import Film from "@/icons/film.svg";
import Panel from "../../core/Panel.vue";

let haveRecordingCount = ref(false);
let recordingCount = ref(0);

function onRecordNew() {
	router.push({ name: "rtTrainRecord" });
}

function showHelp(): boolean {
	return true;// haveRecordingCount.value && recordingCount.value === 0;
}

onMounted(async () => {
	let r = await globals.fetchOrErr('/api/record/count');
	if (!r.ok) {
		return;
	}
	recordingCount.value = parseInt(await r.r.text());
	haveRecordingCount.value = true;
})

</script>

<template>
	<div class="train flexColumnCenter">
		<div v-if="showHelp()" class="helpTopic help">
			Train your system by recording videos that simulate suspicious activities.<br /><br />
			For example, climb over your boundary wall.
		</div>
		<panel>
			<panel-button :icon="RedDot" route-target="rtTrainRecord">New Recording</panel-button>
			<panel-button :icon="Film" route-target="rtTrainEditRecordings">Edit Recordings <span
					v-if="haveRecordingCount">({{
							recordingCount
					}})</span>
			</panel-button>
		</panel>
	</div>
</template>

<style lang="scss" scoped>
.train {
	//margin: 20px 0px 0px 0px;
	padding-top: 20px;
}

.help {
	margin-bottom: 20px;
}
</style>
