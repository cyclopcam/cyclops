import { debugMode } from "./constants";
import { initialScanState, mockScanState, type ScanState } from "./scan";
import { encodeQuery, sleep } from "./util/util";

// SYNC-NATCOM-SERVER
export interface Server {
	lanIP: string;
	publicKey: string;
	bearerToken: string;
	name: string;
}

let debugScanStartedAt = 0;

const fakeServerList = [
	{
		lanIP: "192.168.10.11",
		publicKey: "123456",
		bearerToken: "foobar",
		name: "cyclops",
	},
	{
		lanIP: "192.168.10.15",
		publicKey: "43434343",
		bearerToken: "foobar",
		name: "mars",
	}
];

export function cloneServer(s: Server): Server {
	return {
		lanIP: s.lanIP,
		publicKey: s.publicKey,
		bearerToken: s.bearerToken,
		name: s.name,
	}
}

export function blankServer(): Server {
	return {
		lanIP: "",
		publicKey: "",
		bearerToken: "",
		name: "",
	}
}

export function bestServerName(s: Server): string {
	if (s.name) {
		return s.name;
	}
	if (s.lanIP) {
		return s.lanIP;
	}
	return s.publicKey.substring(0, 6);
}

export async function startScan() {
	if (debugMode) {
		debugScanStartedAt = new Date().getTime();
		return;
	}
	fetch('/natcom/scanForServers', { method: 'POST' });
}

export async function getScanStatus(): Promise<ScanState> {
	if (debugMode) {
		let t = (new Date().getTime() - debugScanStartedAt) / 1000;
		let ss = initialScanState();
		mockScanState(ss, t / 1.5);
		return ss;
	}
	return await (await fetch('/natcom/scanStatus')).json();
}

export async function fetchRegisteredServers() {
	if (debugMode) {
		return fakeServerList;
	}
	let j = await (await fetch("/natcom/getRegisteredServers")).json();
	return j as Server[];
}

export async function switchToRegisteredServer(publicKey: string) {
	await fetch("/natcom/switchToRegisteredServer?" + encodeQuery({ publicKey: publicKey }));
}

export async function getCurrentServer(): Promise<Server> {
	if (debugMode) {
		return fakeServerList[0];
	}
	return await (await fetch("/natcom/getCurrentServer")).json() as Server;
}

export async function showMenu(mode: string) {
	await fetch("/natcom/showMenu?" + encodeQuery({ mode: mode }));
}

export async function getScreenParams(): Promise<{ contentHeight: number }> {
	if (debugMode) {
		// to be truthful, this should be (windowHeight - statusbarHeight) * DPR
		return { contentHeight: 1700 };
	}
	return await (await fetch("/natcom/getScreenParams")).json();
}

export async function setServerProperty(publicKey: string, key: string, value: string) {
	await fetch("/natcom/setServerProperty?" + encodeQuery({ publicKey: publicKey, key: key, value: value }));
}

export async function waitForScreenGrab(): Promise<ImageData> {
	let pauseMS = 5;
	let maxWaitMS = 1000;
	for (let i = 0; i < maxWaitMS / pauseMS; i++) {
		let img = await getScreenGrab(i === 0);
		if (img !== null) {
			return img;
		}
		await sleep(pauseMS);
	}
	throw new Error("Failed to get screen grab");
}

export async function getScreenGrab(forceNew: boolean): Promise<ImageData | null> {
	if (debugMode) {
		let w = 100;
		let h = 200;
		let arr = new Uint8ClampedArray(w * h * 4);
		for (let y = 0; y < h; y++) {
			let p = y * w * 4;
			for (let x = 0; x < w; x++) {
				arr[p] = 255;
				arr[p + 1] = x;
				arr[p + 2] = y;
				arr[p + 3] = 255;
				p += 4;
			}
		}
		return new ImageData(arr, w, h);
	}
	let params = { forceNew: forceNew ? "1" : "0" };
	let grab = await fetch("/natcom/getScreenGrab?" + encodeQuery(params));
	if (grab.status === 202) {
		// server has kicked off a screen grab.
		return null;
	}
	let width = parseInt(grab.headers.get("X-Image-Width")!);
	let height = parseInt(grab.headers.get("X-Image-Height")!);
	let stride = parseInt(grab.headers.get("X-Image-Stride")!); // we ignore stride, so far it's been OK
	let arr = await grab.arrayBuffer();
	let b = new Uint8ClampedArray(arr);
	return new ImageData(b, width, height);
}
