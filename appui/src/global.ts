import { reactive, ref, VueElement } from "vue";
import { initialScanState, mockScanState, mockScanStateError, type ScanState } from "@/scan";

import "@/nativeIn"; // We must import nativeIn *somewhere* so that it's global functions are available
import { blankServer, LocalWebviewVisibility, natFetchRegisteredServers, natGetCurrentServer, natGetScreenGrab, natGetScreenParams, natSetLocalWebviewVisibility, natWaitForScreenGrab, type Server } from "./nativeOut";
import { encodeQuery, sleep } from "@/util/util";
import { panelSlideTransitionMS } from "./constants";
import { pushRoute, replaceRoute } from "./router/routes";

// SYNC-SERVER-PORT
export const ServerPort = 8080;

export class Globals {
	// Most recent LAN scan
	scanState = initialScanState();
	servers: Server[] = [];
	currentServer: Server = blankServer();
	isFullScreen = false; // instruction to App.vue to show BitmapOverlay
	hideFullScreen = false; // instruction to BitmapOverlay to initiate slide-away animation
	fullScreenBackdrop: ImageData | null = null;
	isLoaded = false;
	contentHeight = 0; // Height of our content beneath the status bar, in CSS pixels
	mustShowWelcomeScreen = true; // This state must be sticky, and only disappear once the user is done with initial setup

	constructor() {
		console.log("Globals constructor");
	}

	async startup() {
		await this.loadScreenParams();
		await this.loadServers();

		this.mustShowWelcomeScreen = this.servers.length === 0;

		console.log("isLoaded = true");
		this.isLoaded = true;

		if (this.mustShowWelcomeScreen) {
			// showExpanded will take us to the welcome page
			this.showExpanded(true);
		} else {
			// Note that since we don't expand ourselves in this code path, we'll remain
			// just a status bar on top, and the remote webview will occupy most of the screen.
			console.log("At least one known server, showing rtDefault");
			replaceRoute({ name: 'rtDefault' });
		}
	}

	async loadScreenParams() {
		let sp = await natGetScreenParams();
		this.contentHeight = sp.contentHeight / window.devicePixelRatio;
	}

	async loadServers() {
		try {
			console.log("loadServer start");
			this.servers = await natFetchRegisteredServers();
			let current = await natGetCurrentServer();
			let c = this.servers.find(x => x.publicKey === current.publicKey);
			if (c) {
				this.currentServer = c;
			}
			console.log("loadServer done nServers = ", this.servers.length);
		} catch (e) {
			console.error("loadServer error", e);
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

	async showExpanded(visible: boolean, options: { immediateHide?: boolean, leaveRouteAlone?: boolean } = {}) {
		if (visible) {
			console.log("Enlarge appui");
			// Always set the route back to default when dropping down the menu.
			// This is necessary because we don't have a little back arrow inside this
			// UI. But perhaps we ought to have one...
			if (!options.leaveRouteAlone) {
				if (this.mustShowWelcomeScreen) {
					console.log("No known servers, showing welcome screen");
					replaceRoute({ name: 'rtAddLocal' });
				} else {
					replaceRoute({ name: 'rtDefault' });
				}
			}
			this.fullScreenBackdrop = await natWaitForScreenGrab();
			this.isFullScreen = true; // This starts the creation of BitmapOverlay, which will do natSetLocalWebviewVisibility("2") after being mounted.
			this.hideFullScreen = false;
			natSetLocalWebviewVisibility(LocalWebviewVisibility.PrepareToShow);
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
			await natSetLocalWebviewVisibility(LocalWebviewVisibility.Hidden);
		}
	}

}

export const globals = reactive(new Globals());
