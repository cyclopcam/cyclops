import { randomBytes } from 'crypto';
<script setup lang="ts">
import type { Recording } from '@/recording/recording.js';
import { onMounted, ref, watch } from 'vue';
import Modal from '../../core/Modal.vue';
import Buttin from '../../core/Buttin.vue';
import Play from '@/icons/play-circle.svg';
import Burger from '@/icons/more-vertical.svg';
import SvgButton from '../../core/SvgButton.vue';
import { fetchOrErr, randomString } from '@/util/util';
import Menue from '../../core/Menue.vue';
import { globals } from '@/globals';

let props = defineProps<{
	playerCookie: string, // Used to ensure that there is only one RecordingItem playing a video at a time
	recording?: Recording, // If null, then this is a "record new" pane
	playAtStartup?: boolean,
}>()
let emits = defineEmits(['click', 'playInline', 'delete']);

let showPlayer = ref(false);
let showBurgerMenu = ref(false);
let myCookie = ref('');

let burgerItems = [
	{ action: "delete", title: "Delete" },
	//{ action: "foobar", title: "Delete" },
	//{ action: "zang", title: "Delete" },
]

watch(() => props.playerCookie, (newVal) => {
	if (newVal !== myCookie.value) {
		// another RecordingItem has started playing
		showPlayer.value = false;
	}
})

function showInlinePlayer() {
	showPlayer.value = true;
	myCookie.value = randomString(8);
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

onMounted(() => {
	if (props.playAtStartup) {
		showInlinePlayer();
	}
})

</script>

<template>
	<div :class="{ recording: true, newOuter: !recording, shadow5Hover: !recording }" @click="$emit('click')">
		<div v-if="recording" class="imgContainer">
			<img v-if="!showPlayer" :src="'/api/record/thumbnail/' + recording.id" class="shadow5" loading="lazy" />
			<svg-button v-if="!showPlayer" :icon="Play" icon-size="38px" class="playBtn" :invert="true" :shadow="true"
				@click="showInlinePlayer" />
			<video v-if="showPlayer" :src="'/api/record/video/LD/' + recording.id" class="inlineVideo" autoplay
				controls />
			<svg-button :icon="Burger" icon-size="36px" class="burgerBtn" :invert="true" :shadow="true"
				@click="showBurgerMenu = true" />
			<menue v-if="showBurgerMenu" :items="burgerItems" @close="showBurgerMenu = false"
				@select="onBurgerSelect" />
		</div>
		<!--
		<modal v-if="recording && showPlayer" @close="showPlayer = false" position="previous" :poll-size="true"
			:click-through="true">
			<video :src="'/api/record/video/LD/' + recording.id" class="popupVideo" />
		</modal>
		-->

		<div v-if="!recording" class="flexCenter new">
			New Recording
		</div>
	</div>
</template>

<style lang="scss" scoped>
@import '@/assets/vars.scss';

.recording {
	position: relative;
	width: 200px;
	height: 150px;

	@media (max-width: $mobileCutoff) {
		// fit two per row
		width: 160px;
		height: 120px;
	}
}

.imgContainer {
	width: 100%;
	height: 100%;
	position: relative;
}

img {
	width: 100%;
	height: 100%;
	border-radius: 3px;
}

.playBtn {
	position: absolute;
	left: 0px;
	top: 0px;
	//padding: 4px;
}

.burgerBtn {
	position: absolute;
	right: 0px;
	top: 0px;
}

.newOuter {
	@include flexCenter();
	cursor: pointer;
	border-radius: 3px;
}

.new {
	border: dashed 2px #ddd;
	border-radius: 10px;
	padding: 6px 12px;
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
