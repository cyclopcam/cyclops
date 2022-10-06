<script setup lang="ts">
import type { Recording } from '@/recording/recording';
import { onMounted, ref } from 'vue';
import Play from '@/icons/play-circle.svg';
import Burger from '@/icons/more-vertical.svg';
import SvgButton from '../../core/SvgButton.vue';
import { dateTimeShort, fetchOrErr, randomString } from '@/util/util';
import Menue from '../../core/Menue.vue';
import { globals } from '@/globals';
import SelectButton from '../../core/SelectButton.vue';

let props = defineProps<{
	playerCookie: string, // Used to ensure that there is only one RecordingItem playing a video at a time
	recording: Recording,
	selection?: Set<number>,
	playAtStartup?: boolean,
}>()
let emits = defineEmits(['click', 'playInline', 'delete', 'openLabeler']);

let enableBurgerMenu = ref(false);
let showBurgerMenu = ref(false);
let myCookie = ref(randomString(8));

let burgerItems = [
	{ action: "delete", title: "Delete" },
	//{ action: "foobar", title: "Delete" },
	//{ action: "zang", title: "Delete" },
]

function showPlayer(): boolean {
	return myCookie.value === props.playerCookie;
}

function labelTxt(): string {
	return 'unlabeled';
}

function showInlinePlayer() {
	// coordinate with owner so that there is only one player at a time
	emits('playInline', myCookie.value);
}

async function onBurgerSelect(item: typeof burgerItems[0]) {
	if (item.action === 'delete') {
		let r = await fetchOrErr("/api/record/delete/" + props.recording!.id, { method: "POST" });
		if (!r.ok) {
			globals.networkError = r.error;
			return;
		}
		emits('delete');
	}
}

function startDate(): string {
	return dateTimeShort(props.recording!.startTime);
}

function onSelect(v: boolean) {
	if (!props.selection) {
		return;
	}

	if (v) {
		props.selection.add(props.recording.id);
	} else {
		props.selection.delete(props.recording.id);
	}
}

onMounted(() => {
	if (props.playAtStartup) {
		showInlinePlayer();
	}
})

</script>

<template>
	<div class="recording" @click="$emit('click')">
		<div class="imgContainer">
			<img v-if="!showPlayer()" :src="'/api/record/thumbnail/' + recording.id" class="shadow5L" loading="lazy" />
			<video v-if="showPlayer()" :src="'/api/record/video/LD/' + recording.id" class="inlineVideo" autoplay
				controls />
			<div v-if="!showPlayer()" class="overlayButtonContainer">
				<svg-button v-if="!showPlayer()" :icon="Play" size="38px" class="playBtn" :invert="true" :shadow="true"
					@click="showInlinePlayer" />
				<svg-button v-if="enableBurgerMenu" :icon="Burger" size="36px" class="burgerBtn" :invert="true"
					:shadow="true" @click="showBurgerMenu = true" />
				<menue v-if="showBurgerMenu" :items="burgerItems" @close="showBurgerMenu = false"
					@select="onBurgerSelect" />
			</div>
		</div>
		<div class="bottomSection">
			<div style="font-size:12px">
				{{ startDate() }}
			</div>
			<div class="labelTxt shadow5LHover" @click="$emit('openLabeler')">
				{{ labelTxt() }}
			</div>
			<select-button v-if="selection" :model-value="selection.has(recording.id)" @update:model-value="onSelect" />
		</div>
	</div>
</template>

<style lang="scss" scoped>
@import '@/assets/vars.scss';

.recording {
	position: relative;
	@include shadow5;
	padding: 10px;
	border-radius: 3px;
	//width: 200px;
	//height: 150px;

	//@media (max-width: $mobileCutoff) {
	//	// fit two per row
	//	width: 160px;
	//	height: 120px;
	//}
}

.imgContainer {
	//width: 100%;
	//height: 100%;
	width: 280px;
	height: 195px;
	position: relative;
}

.bottomSection {
	margin: 10px 0 0 0;
	font-size: 14px;
	display: flex;
	justify-content: space-between;
	align-items: center;
}

.labelTxt {
	user-select: none;
	font-size: 12px;
	border: solid 1px #ddd;
	padding: 3px 5px;
	border-radius: 3px;
	background-color: rgba(200, 200, 200, 0.2);
	max-width: 80px;
	overflow-x: hidden;
	text-overflow: ellipsis;
	cursor: pointer;
}

img {
	width: 100%;
	height: 100%;
	border-radius: 3px;
}

.overlayButtonContainer {
	position: absolute;
	left: 0;
	top: 0;
	width: 100%;
	height: 100%;
	@include flexCenter();
}

.playBtn {
	//position: absolute;
	//left: 0px;
	//top: 0px;
	//padding: 4px;
}

.burgerBtn {
	position: absolute;
	right: 0px;
	top: 0px;
}

.inlineVideo {
	width: 100%;
	height: 100%;
	//border: solid 2px #333;
	//border-radius: 3px;
	//box-shadow: 5px 5px 15px rgba(0, 0, 0, 0.5);
	object-fit: fill;
}
</style>
