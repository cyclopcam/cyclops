import { createRouter, createWebHistory } from "vue-router";
import HomeView from "../views/HomeView.vue";
import WelcomeView from "../views/WelcomeView.vue";
import LoginView from "../views/LoginView.vue";
import Settings from "@/components/home/Settings.vue";
import Monitor from "@/components/home/Monitor.vue";
import Alarm from "@/components/home/Alarm.vue";
import Train from "@/components/home/train/Train.vue";
import TrainHome from "@/components/home/train/TrainHome.vue";
import TrainRecord from "@/components/home/train/Recorder.vue";
import TrainEditRecordings from "@/components/home/train/EditRecordings.vue";
import SettingsHome from "@/components/settings/SettingsHome.vue";
import EditCamera from "@/components/settings/EditCamera.vue";
import ScanForCameras from "@/components/settings/ScanForCameras.vue";
import AddCamera from "@/components/settings/AddCamera.vue";
import SystemSettings from "@/components/settings/SystemSettings.vue";
import EditDetectionZone from "@/components/settings/EditDetectionZone.vue";
import Empty from "@/components/home/Empty.vue";
import BlankView from "@/views/Blank.vue";

export const router = createRouter({
	history: createWebHistory(import.meta.env.BASE_URL),
	routes: [
		{
			path: "/",
			name: "rtHome",
			component: HomeView,
			children: [
				{
					path: "settings",
					name: "rtSettings",
					component: Settings,
					children: [
						{
							path: "",
							name: "rtSettingsHome",
							component: SettingsHome,
						},
						{
							path: "system",
							name: "rtSettingsSystem",
							component: SystemSettings,
						},
						{
							path: "camera/:id",
							name: "rtSettingsEditCamera",
							component: EditCamera,
							props: true,
						},
						{
							path: "camera/:id/detectionZone",
							name: "rtSettingsEditDetectionZone",
							component: EditDetectionZone,
							props: true,
						},
						{
							path: "scan",
							name: "rtSettingsScanForCameras",
							component: ScanForCameras,
						},
						{
							path: "addCamera",
							name: "rtSettingsAddCamera",
							component: AddCamera,
						},
					],
				},
				{
					path: "monitor",
					name: "rtMonitor",
					component: Monitor,
				},
				{
					path: "alarm",
					name: "rtAlarm",
					component: Alarm,
				},
				{
					path: "train",
					name: "rtTrain", // We never navigate to this, but it is used to show the TopBar toggle button
					component: Train,
					children: [
						{
							path: "",
							name: "rtTrainHome",
							component: TrainHome,
						},
						{
							path: "record",
							name: "rtTrainRecord",
							component: TrainRecord,
						},
						{
							path: "edit",
							name: "rtTrainEditRecordings",
							component: TrainEditRecordings,
						},
						{
							path: "label/:id",
							props: true,
							name: "rtTrainLabelRecording",
							component: TrainEditRecordings,
						},
						//{
						//	path: "edit/:recordingID",
						//	name: "rtTrainLabelRecording",
						//	component: TrainLabeler,
						//	props: true,
						//},
					],
				},
				{
					path: "empty",
					name: "rtEmpty",
					component: Empty,
				},
			],
		},
		{
			path: "/welcome",
			name: "rtWelcome",
			component: WelcomeView,
		},
		{
			path: "/login",
			name: "rtLogin",
			component: WelcomeView, // It made sense to just subsume Welcome and Login into one page
		},
		{
			path: "/blank",
			name: "rtBlank",
			component: BlankView,
		},
		{
			path: "/about",
			name: "rtAbout",
			// route level code-splitting
			// this generates a separate chunk (About.[hash].js) for this route
			// which is lazy-loaded when the route is visited.
			component: () => import("../views/AboutView.vue"),
		},
	],
});

router.afterEach((to, from) => {
	//console.log("Route", router.currentRoute.value);

	// Handle UI transitions (swipe left/right animations)
	const toDepth = to.path.split("/").length;
	const fromDepth = from.path.split("/").length;
	to.meta.transitionName = toDepth < fromDepth ? "slide-right" : "slide-left";
});



