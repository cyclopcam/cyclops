<script setup lang="ts">
import { OntologyLevel, OntologyTag, type Ontology, type Recording, Labels } from '@/recording/recording';
import { encodeQuery } from '@/util/util';
import { onMounted, ref } from 'vue';
import VideoTimeline from '../../core/VideoTimeline.vue';
import Cropper from '../../core/Cropper.vue';
import Tag from './Tag.vue';
import InfoBubble from '../../widgets/InfoBubble.vue';
import LevelsExplainer from './LevelsExplainer.vue';

// It was too painful to make this a true top-level route,
// so I moved it back to a child component, where we bring
// recording in through a prop.

let props = defineProps<{
	recording: Recording,
	ontologies: Ontology[],
	latestOntology: Ontology,
}>()

let emits = defineEmits(['close']);

const defaultDuration = 1.0; // use 1.0 instead of 0 so we don't have to worry about div/0

let video = ref(null);

let duration = ref(defaultDuration);
let cropStart = ref(0);
let cropEnd = ref(defaultDuration);
let seekPosition = ref(0);
let videoTag = ref(new OntologyTag("", OntologyLevel.Record)); // Blank name === nothing (by step() function)
let haveCrop = ref(false);
let isFreshLabel = ref(true); // True if this is the first time that the user is labelling this video (i.e. there was no existing labelling data for it)
let useVideo = ref(false);
let error = ref("");

function videoElement(): HTMLVideoElement {
	return video.value! as HTMLVideoElement
}

function videoURL(): string {
	return '/api/record/video/LD/' + props.recording.id + '?' + encodeQuery({ 'seekable': '1' });
}

function ontology(): Ontology {
	if (props.recording.ontologyID) {
		let o = props.ontologies.find(o => o.id === props.recording.ontologyID);
		if (o) {
			return o;
		} else {
			console.error(`Ontology ${props.recording.ontologyID} not found}`);
		}
	}
	return props.latestOntology;
}

function orderedTags(): OntologyTag[] {
	let tags = ontology().tags.slice();
	tags.sort((a, b) => b.severity - a.severity);
	return tags;
}

enum Steps {
	Event,
	Crop,
	UseVideo,
	Done,
}

function step(): Steps {
	if (!isFreshLabel.value) {
		return Steps.Done;
	}

	if (videoTag.value.name === "") {
		return Steps.Event;
	}
	if (!haveCrop.value) {
		return Steps.Crop;
	}
	if (!useVideo.value) {
		return Steps.UseVideo;
	}
	return Steps.Done;
}

function onSeek(t: number) {
	seekPosition.value = t;
	videoElement().currentTime = t;
}

function onCropStart(v: number) {
	cropStart.value = v;
	onCropAny();
	onSeek(v);
}

function onCropEnd(v: number) {
	cropEnd.value = v;
	onCropAny();
	onSeek(v);
}

function onCropAny() {
	haveCrop.value = true;
	if (isFreshLabel.value) {
		useVideo.value = true;
	}
}

function onLoadVideoData() {
	//console.log("onLoadVideoData", videoElement().duration);
}

function onLoadVideoMetadata() {
	let el = videoElement();
	//console.log("onLoadVideoMetadata", el.duration, cropStart.value, cropEnd.value);
	if (!isNaN(el.duration)) {
		duration.value = el.duration;
		seekPosition.value = 0;
		if (!haveCrop.value) {
			cropStart.value = el.duration / 4;
			cropEnd.value = el.duration * 3 / 4;
			console.log("reset crops to ", cropStart.value, cropEnd.value);
		}
		onSeek(0);
	}
}

function onContextMenu(ev: Event) {
	//console.log("onContextMenu");
	// This is vital to prevent annoying context menu popups on long press on mobile
	ev.preventDefault();
	//return false;
}

function onTagSelect(tag: OntologyTag) {
	videoTag.value = tag;
}

function onCancel() {
	emits('close');
}

function canSave(): boolean {
	return step() >= Steps.Done || !isFreshLabel.value;
}

async function onSave() {
	//console.log("Labeler onSave");
	props.recording.labels = new Labels();
	props.recording.labels.videoTags.push(ontology().tags.indexOf(videoTag.value));
	props.recording.labels.cropStart = Math.round(cropStart.value * 100) / 100;
	props.recording.labels.cropEnd = Math.round(cropEnd.value * 100) / 100;
	props.recording.useForTraining = useVideo.value;
	props.recording.ontologyID = ontology().id;

	let r = await props.recording.saveLabels();
	if (!r.ok) {
		error.value = r.error;
	} else {
		emits('close');
	}
}

function loadLabels() {
	//console.log("loadLabels", props.recording.labels);
	let labels = props.recording.labels;
	let o = ontology();
	if (!labels || !o) {
		return;
	}
	isFreshLabel.value = false;
	if (labels.videoTags.length > 0) {
		videoTag.value = o.tags[labels.videoTags[0]];
	}
	haveCrop.value = true;
	cropStart.value = labels.cropStart;
	cropEnd.value = labels.cropEnd;
	useVideo.value = props.recording.useForTraining;
}

onMounted(async () => {
	//console.log("duration", videoElement().duration);
	loadLabels();
})

</script>

<template>
	<div class="labelerRoot" @contextmenu="onContextMenu">
		<video v-if="recording.id !== 0" ref="video" :src="videoURL()" class="video" style="position: relative"
			@loadedmetadata="onLoadVideoMetadata" @loadeddata="onLoadVideoData" />
		<video-timeline class="timeline" :transparent="true" :duration="duration" :seek-position="seekPosition"
			@seek="onSeek" />

		<div class="form">
			<div style="height:25px" />
			<div :class="{ instruction: true, nextStep: step() === Steps.Event }">What is happening in this video?</div>
			<div class="flex tagListContainer">
				<div class="flexColumn tagList">
					<tag v-for="tag of orderedTags()" :tag="tag" :selectable="true" :selected="videoTag === tag"
						@select="onTagSelect(tag)" />
				</div>
				<div class="tagHelpPanel">
					<info-bubble caption="Explain levels" tint="mild">
						<levels-explainer />
					</info-bubble>
				</div>
			</div>

			<div style="height:25px" />
			<div :class="{ cropContainer: true, unavailable: step() < Steps.Crop, available: step() >= Steps.Crop }">
				<div :class="{ instruction: true, nextStep: step() === Steps.Crop }">
					Crop the video to the precise moments when this occurs</div>
				<cropper class="cropper" :duration="duration" :start="cropStart" :end="cropEnd" @seek-start="onCropStart"
					@seek-end="onCropEnd" />
				<div style="height:15px" />
			</div>

			<div style="height:15px" />
			<label
				:class="{ checkboxLabel: true, unavailable: step() < Steps.UseVideo, available: step() >= Steps.UseVideo, nextStep: step() === Steps.UseVideo }">
				<input type="checkbox" v-model="useVideo" />Use this video to train my system
			</label>

			<div style="height:20px" />
			<div v-if="error !== ''" class="error">{{ error }}</div>
			<div class="bottomPanel">
				<!-- <svg-button :icon="Back" size="30px" @click="onBack" /> -->
				<button @click="onCancel">Cancel</button>
				<div :class="{ finish: true, unavailable: !canSave(), available: canSave(), nextStep: canSave() }">
					<!-- <svg-button :icon="Next" size="40px" @click="onDone" /> -->
					<button class="focalButton" @click="onSave">Save</button>
				</div>
			</div>
		</div>
	</div>
</template>

<style lang="scss" scoped>
@import '@/assets/vars.scss';


.labelerRoot {
	display: flex;
	flex-direction: column;
	align-items: center;
	position: relative;

	box-sizing: border-box;
	background-color: #fff;
	padding: 20px 10px 30px 10px;

	width: 400px;

	@media (max-width: $mobileCutoff) {
		width: 100%;
		height: 100%;
	}
}

$videoWidth: 340px;
$videoHeight: 250px;

.video {
	width: $videoWidth;
	height: $videoHeight;
	object-fit: fill;
	border-radius: 2px;
}

.form {
	display: flex;
	flex-direction: column;
}

.instruction {
	//font-weight: 500;

	font-size: 16px;
	margin-bottom: 12px;

	@media (max-width: $mobileCutoff) {
		font-size: 18px;
		margin-bottom: 14px;
	}

	color: #000;
	transition: color 0.5s;
}

.nextStep {
	color: #00a;
	font-weight: 600;
}

.tagListContainer {
	//align-items: center;
	align-self: flex-start;
	//margin-left: 10px;
}

.tagList {
	//width: $videoWidth;
	box-sizing: border-box;
	padding-left: 10px;
	gap: 8px;

	@media (max-width: $mobileCutoff) {
		gap: 12px;
	}
}

.tagHelpPanel {
	margin-left: 50px;
	@include flexCenter();
}

.cropContainer {
	width: $videoWidth;
	// center, so that our crop control matches the seek bar in the video...
	// I'm beginning to wonder if it's worth syncing those two.
	display: flex;
	flex-direction: column;
	align-items: center;
}

input[type=checkbox] {
	width: 18px;
	height: 18px;
	margin-right: 8px;
}

.checkboxLabel {
	display: flex;
	align-items: center;
}

$timelineWidth: calc($videoWidth - 20px);

.cropper {
	width: $timelineWidth;
}

.timeline {
	width: $timelineWidth;
}

.error {
	margin: 10px 0px;
	color: #d00;
}

.bottomPanel {
	display: flex;
	//justify-content: space-between;
	justify-content: flex-end;
	gap: 10px;
}

button {
	font-weight: 600;
}

.finish {
	display: flex;
	//justify-content: flex-end;
}
</style>
