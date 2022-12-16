// nativeOut has functions that we use to talk to our native (Java/Swift) component

import { dummyMode } from "./constants";
import { initialScanState, mockScanState, type ScanState } from "./scan";
import { encodeQuery, sleep } from "./util/util";
import type { ScannedServer } from '@/scan';
import { ServerPort } from "./global";

// SYNC-NATCOM-SERVER
export interface Server {
	lanIP: string;
	publicKey: string;
	bearerToken: string;
	name: string;
}

let debugScanStartedAt = 0;

export const fakeServerList: Server[] = [
	{
		lanIP: "192.168.10.11",
		publicKey: "MCUAwePTQX3/K8LTXEAAyFJHp9dyzF8Z/tDRWvsfd10=",
		bearerToken: "foobar1",
		name: "venus",
	},
	{
		lanIP: "192.168.10.15",
		publicKey: "sORQPp3+0Gz/16uU4kq9IlE5AlU50IRYBLRXGkyZHV0=",
		bearerToken: "foobar2",
		name: "mars",
	},
	{
		lanIP: "192.168.10.69",
		publicKey: "0Ht1CfNitWlTZeXfxK9I9Wy0W8ur29CrbDhx5tNKrWU=",
		bearerToken: "foobar3",
		name: "molehill",
	}
];

// Setup a bunch of servers to simulate a system that is already configured
export let registeredFakeServers = fakeServerList.filter(x => x.name === "venus" || x.name === "mars");

// Use an empty list to debug the welcome screen
//registeredFakeServers = [];

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

export async function natStartScan() {
	if (dummyMode) {
		debugScanStartedAt = new Date().getTime();
		return;
	}
	fetch('/natcom/scanForServers', { method: 'POST' });
}

export async function natGetScanStatus(): Promise<ScanState> {
	if (dummyMode) {
		let t = (new Date().getTime() - debugScanStartedAt) / 1000;
		let ss = initialScanState();
		mockScanState(ss, t / 1.5);
		return ss;
	}
	return await (await fetch('/natcom/scanStatus')).json();
}

export async function natFetchRegisteredServers() {
	if (dummyMode) {
		return registeredFakeServers;
	}
	let j = await (await fetch("/natcom/getRegisteredServers")).json();
	return j as Server[];
}

export async function natSwitchToRegisteredServer(publicKey: string) {
	await fetch("/natcom/switchToRegisteredServer?" + encodeQuery({ publicKey: publicKey }));
}

export async function natGetLastServer(): Promise<Server> {
	if (dummyMode) {
		return registeredFakeServers.length === 0 ? blankServer() : registeredFakeServers[0];
	}
	return await (await fetch("/natcom/getLastServer")).json() as Server;
}

export enum LocalWebviewVisibility {
	Hidden = "0",
	PrepareToShow = "1",
	Show = "2",
}

// mode is 0,1,2. See native code.
export async function natSetLocalWebviewVisibility(mode: LocalWebviewVisibility) {
	await fetch("/natcom/setLocalWebviewVisibility?" + encodeQuery({ mode: mode }));
}

export async function natGetScreenParams(): Promise<{ contentHeight: number }> {
	if (dummyMode) {
		// The 40 here is the status bar height
		// SYNC-STATUS-BAR-HEIGHT
		return { contentHeight: (window.innerHeight - 40) * window.devicePixelRatio };
		//return { contentHeight: 1900 };
	}
	return await (await fetch("/natcom/getScreenParams")).json();
}

export async function natSetServerProperty(publicKey: string, key: string, value: string) {
	await fetch("/natcom/setServerProperty?" + encodeQuery({ publicKey: publicKey, key: key, value: value }));
}

export async function natNavigateToScannedLocalServer(s: ScannedServer) {
	//let baseUrl = `http://${s.ip}:${ServerPort}`;
	//await fetch('/natcom/navigateToScannedLocalServer?' + encodeQuery({ url: baseUrl }));
	await fetch('/natcom/navigateToScannedLocalServer?' + encodeQuery({ publicKey: s.publicKey }));
}

export async function natWaitForScreenGrab(): Promise<ImageData> {
	let start = new Date().getTime();
	let pauseMS = 5;
	let maxWaitMS = 2000;
	for (let i = 0; i < maxWaitMS / pauseMS; i++) {
		let img = await natGetScreenGrab(i === 0);
		if (img !== null) {
			console.log("natWaitForScreenGrab took " + (new Date().getTime() - start) + " ms");
			return img;
		}
		await sleep(pauseMS);
	}
	throw new Error("Failed to get screen grab");
}

export async function natGetScreenGrab(forceNew: boolean): Promise<ImageData | null> {
	if (dummyMode) {
		// natGetScreenParams() is the place where the height of this bitmap is controlled
		let w = 100;
		let h = 600;
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
