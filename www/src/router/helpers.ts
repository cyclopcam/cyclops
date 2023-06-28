import type { RouteLocationRaw, Router } from "vue-router";

// We need to keep track of the history stack size, so that we can synchronously return "false"
// from cyBack(), which will inform the host application that it must issue an Activity-level "back",
// which will usually exit the program.
// Vue router doesn't give us access to this state, which is why we need to keep track of it ourselves.
export let routeStackSize = 0;

export function pushRoute(router: Router, to: RouteLocationRaw) {
	routeStackSize++;
	router.push(to);
}

export function popRoute(router: Router): boolean {
	if (routeStackSize <= 0) {
		return false;
	}
	routeStackSize--;
	router.back();
	return true;
}

export function replaceRoute(router: Router, to: RouteLocationRaw) {
	routeStackSize = 0;
	router.replace(to);
}
