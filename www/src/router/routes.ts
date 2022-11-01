import { createRouter, createWebHistory, type RouteLocationRaw } from "vue-router";
import HomeView from "../views/HomeView.vue";
import WelcomeView from "../views/WelcomeView.vue";
import LoginView from "../views/LoginView.vue";
import Settings from "@/components/home/Settings.vue";
import Monitor from "@/components/home/Monitor.vue";
import Train from "@/components/home/train/Train.vue";
import TrainHome from "@/components/home/train/TrainHome.vue";
import TrainRecord from "@/components/home/train/Recorder.vue";
import TrainEditRecordings from "@/components/home/train/EditRecordings.vue";
//import TrainLabeler from "@/components/home/train/Labeler.vue";
import SettingsTop from "@/components/settings/SettingsTop.vue";
import SetupCameras from "@/components/settings/SetupCameras.vue";
import SystemVariables from "@/components/settings/SystemVariables.vue";
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
							name: "rtSettingsTop",
							component: SettingsTop,
						},
						{
							path: "cameras",
							name: "rtSettingsCameras",
							component: SetupCameras,
						},
						{
							path: "system",
							name: "rtSettingsSystem",
							component: SystemVariables,
						},
					],
				},
				{
					path: "monitor",
					name: "rtMonitor",
					component: Monitor,
				},
				{
					path: "train",
					name: "rtTrain", // We never navigate to this, but it is used to show the TopBar toggle buton
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
							path: "edit/:id",
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
			component: LoginView,
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
	const toDepth = to.path.split("/").length;
	const fromDepth = from.path.split("/").length;
	to.meta.transitionName = toDepth < fromDepth ? "slide-right" : "slide-left";
});

// We need to keep track of the history stack size, so that we can synchronously return "false"
// from cyBack(), which will inform the host application that it must issue an Activity-level "back",
// which will usually exit the program.
// Vue router doesn't give us access to this state, which is why we need to keep track of it ourselves.
export let routeStackSize = 0;

export function replaceRoute(to: RouteLocationRaw) {
	routeStackSize = 0;
	router.replace(to);
}

export function pushRoute(to: RouteLocationRaw) {
	routeStackSize++;
	router.push(to);
}

export function popRoute(): boolean {
	if (routeStackSize <= 0) {
		return false;
	}
	routeStackSize--;
	router.back();
	return true;
}
