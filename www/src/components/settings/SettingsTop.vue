<script setup lang="ts">
import PanelButton from "../core/PanelButton.vue";
import Panel from "../core/Panel.vue";
import video from "@/icons/video.svg";
import server from "@/icons/server.svg";
</script>

<template>
	<panel>
		<panel-button :icon="video" icon-size="18px" route-target="rtSettingsCameras">Cameras</panel-button>
		<panel-button :icon="server" icon-size="18px" route-target="rtSettingsSystem">System Settings</panel-button>
	</panel>
</template>
