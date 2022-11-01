import { fetchOrErr } from "./util/util";

// Random bag of constants from the server
export interface Constants {
	cameraModels: string[];
}

// constants is initialized before the Vue application is loaded, so it's always available
export let constants: Constants;

// load constants from server, and if server is unreachable, then load from localStorage
// The localStorage thing is just to make the rest of the code in this app simpler, so that
// doesn't ever have to think about constants being null.
export async function loadConstants() {
	let c = await fetchOrErr("/api/system/constants");
	if (c.ok) {
		constants = await c.r.json();
		localStorage.setItem("constants", JSON.stringify(constants));
	} else {
		let ls = localStorage.getItem("constants");
		if (ls) {
			constants = JSON.parse(ls);
		}
	}
}
