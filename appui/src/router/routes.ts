import { createMemoryHistory, createRouter, createWebHashHistory, createWebHistory, type RouteLocationRaw } from "vue-router";
import AddLocal from "@/components/AddLocal.vue";
import ConnectExisting from "@/components/ConnectExisting.vue";
//import StatusBar from "@/components/StatusBar.vue";
import Blank from "@/components/Blank.vue";
import Default from "@/components/Default.vue";
import EditServer from "@/components/EditServer.vue";
import { debugMode } from "@/constants";

export const router = createRouter({
	// WebHistory doesn't work with our WebView.
	// MemoryHistory works for our WebView, but I haven't bothered to understand how it differs from WebHash
	history: debugMode ? createWebHistory(import.meta.env.BASE_URL) : createMemoryHistory(),
	//history: createWebHashHistory("https://appassets.androidplatform.net/assets/index.html"),
	//history: createWebHistory(import.meta.env.BASE_URL),
	routes: [
		{
			path: "/addLocal/:init/:scanOnLoad",
			name: "rtAddLocal",
			component: AddLocal,
			meta: { depth: 1 },
			props: true,
		},
		//{
		//	path: "/local/:ip/:host",
		//	name: "rtConnectLocal",
		//	component: ConnectLocal,
		//	meta: { depth: 2 },
		//	props: true
		//},
		{
			path: "/existing",
			name: "rtConnectExisting",
			component: ConnectExisting,
			meta: { depth: 2 },
		},
		//{
		//	path: "/status",
		//	name: "rtStatusBar",
		//	component: StatusBar,
		//	meta: { depth: -1 },
		//},
		{
			path: "/blank",
			name: "rtBlank",
			component: Blank,
			meta: { depth: -1 },
		},
		{
			path: "/default",
			name: "rtDefault",
			component: Default,
			meta: { depth: 1 },
		},
		{
			path: "/editServer/:publicKey",
			name: "rtEditServer",
			component: EditServer,
			meta: { depth: 2 },
			props: true,
		},
	],
});

router.afterEach((to, from) => {
	//console.log("Route", router.currentRoute.value);
	//const toDepth = to.path.split("/").length;
	//const fromDepth = from.path.split("/").length;
	const toDepth = to.meta.depth as number;
	const fromDepth = from.meta.depth as number;
	if (toDepth < 0 || fromDepth < 0) {
		// negative depth means "do not transition"
		to.meta.transitionName = "";
		return;
	}
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
