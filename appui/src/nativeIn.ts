// nativeIn exposes entrypoints that our native app (Java/Swift) uses to talk to us.

import { globals } from "./global";
import { popRoute, router } from "./router/routes";

// Set the route/page
(window as any).cySetRoute = (route: string, params?: { [key: string]: string }) => {
	console.log("cySetRoute", route, params ? JSON.stringify(params) : "<no params>");
	router.replace({ name: route, params: params ?? undefined });
};

// Ask us to refresh our list of servers
(window as any).cyRefreshServers = () => {
	console.log("cyRefreshServers");
	globals.loadServers();
	if (globals.servers.length !== 0) {
		globals.mustShowWelcomeScreen = false;
	}
};

// Back/Forward in history
(window as any).cyBack = () => {
	if (!popRoute()) {
		return false;
	}
	return true;
};
