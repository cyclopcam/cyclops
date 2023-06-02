import { reactive } from "vue";
import { fetchOrErr } from "./util/util";

// Random bag of constants from the server
export interface Constants {
	cameraModels: string[];
}

// constants is initialized before the Vue application is loaded, so it's always available
export let constants: Constants = reactive({
	cameraModels: [],
});

function setConstantsFromJSON(j: Constants) {
	// We must set each member of 'constants' individually.
	// If we just do constants = reactive(j), then Vue will not notice that the object has changed.
	// I find this surprising.
	constants.cameraModels = j.cameraModels;
}

// load constants from server, and if server is unreachable, then load from localStorage
// The localStorage thing is just to make the rest of the code in this app simpler, so that
// we don't ever have to think about constants being null.
export async function loadConstants() {
	let c = await fetchOrErr("/api/system/constants");
	if (c.ok) {
		let local = await c.r.json();
		localStorage.setItem("constants", JSON.stringify(local));
		setConstantsFromJSON(local);
	} else {
		let ls = localStorage.getItem("constants");
		if (ls) {
			setConstantsFromJSON(JSON.parse(ls));
		}
	}
}

