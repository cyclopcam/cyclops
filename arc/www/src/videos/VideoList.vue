<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { Video } from "@/videos/video";
import VideoThumbnail from '@/videos/VideoThumbnail.vue';
import VideoEdit from '@/videos/VideoEdit.vue';
import Modal from '@/widgets/Modal.vue';

let videos = ref([] as Video[]);
let edit = ref(null as Video | null);

function onClickThumbnail(video: Video) {
	edit.value = video;
}

onMounted(async () => {
	videos.value = await Video.fetchAll();
});

</script>

<template>
	<div class="videoList">
		<VideoThumbnail v-for="video in videos" :key="video.id" :video="video" @click="onClickThumbnail(video)" />
		<Modal v-if="edit" :show-x="true" @close="edit = null" tint="dark">
			<VideoEdit :video="edit" />
		</Modal>
	</div>
</template>

<style scoped lang="scss">
.videoList {
	display: flex;
	flex-wrap: wrap;
	gap: 20px;
}
</style>
