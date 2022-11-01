import { popRoute, router } from "./router/routes";

// natcom stands for Native Comms - i.e. communication with the native (Java/ObjC/etc) side

// Set the page
(window as any).cySetRoute = (route: string) => {
	router.replace({ name: route });
};

// Back/Forward in history
(window as any).cyBack = () => {
	if (!popRoute()) {
		return false;
	}
	return true;
};
