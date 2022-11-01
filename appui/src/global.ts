import { reactive, ref, VueElement } from "vue";
import { initialScanState, mockScanState, mockScanStateError, type ScanState } from "@/scan";

import "@/natcom"; // We must import natcom *somewhere* so that it's global functions are available
import { blankServer, fetchRegisteredServers, getCurrentServer, getScreenGrab, getScreenParams, showMenu, waitForScreenGrab, type Server } from "./nattypes";
import { encodeQuery, sleep } from "@/util/util";
import { panelSlideTransitionMS } from "./constants";

// SYNC-SERVER-PORT
export const ServerPort = 8080;

export class Globals {
	// Most recent LAN scan
	scanState = initialScanState();
	servers: Server[] = [];
	currentServer: Server = blankServer();
	isFullScreen = false;
	hideFullScreen = false; // instruction to BitmapOverlay to initiate slide-away animation
	fullScreenBackdrop: ImageData | null = null;
	isLoaded = false;
	contentHeight = 0; // in CSS pixels

	constructor() {
		//mockScanState(this.scanState);
		//mockScanStateError(this.scanState);
		this.loadScreenParams();
		this.loadServers();
	}

	async loadScreenParams() {
		let sp = await getScreenParams();
		this.contentHeight = sp.contentHeight / window.devicePixelRatio;
	}

	async loadServers() {
		try {
			console.log("loadServer start");
			this.servers = await fetchRegisteredServers();
			let current = await getCurrentServer();
			let c = this.servers.find(x => x.publicKey === current.publicKey);
			if (c) {
				this.currentServer = c;
			}
			console.log("loadServer done nServers = ", this.servers.length);
		} finally {
			console.log("isLoaded = true");
			this.isLoaded = true;
		}
	}

	async waitForLoad() {
		let pauseMS = 10;
		let waitMS = 2000;
		for (let i = 0; i < waitMS / pauseMS; i++) {
			if (this.isLoaded) {
				return;
			}
			await sleep(pauseMS);
		}
	}

	async showMenu(visible: boolean, options: { immediateHide?: boolean } = {}) {
		if (visible) {
			console.log("Enlarge appui");
			this.fullScreenBackdrop = await waitForScreenGrab();
			this.isFullScreen = true;
			this.hideFullScreen = false;
			showMenu("1");
			// wait for a vue/browser layout, and then actually expand the WebView on the Android side
			//setTimeout(() => {
			//	showMenu("1");
			//}, 0);
			//setTimeout(() => {
			//	showMenu("2");
			//}, 50);
		} else {
			console.log("Shrink appui");
			if (options?.immediateHide) {
				this.fullScreenBackdrop = null;
				this.isFullScreen = false;
			} else {
				// Tell BitmapOverlay to start swiping it's content away
				this.hideFullScreen = true;
				setTimeout(() => {
					// BitmapOverlay's animation is done, so remove our dropdown content, and show only our StatusBar
					this.fullScreenBackdrop = null;
					this.isFullScreen = false;
				}, panelSlideTransitionMS);
			}
			await showMenu("0");
		}
	}

}

export const globals = reactive(new Globals());
