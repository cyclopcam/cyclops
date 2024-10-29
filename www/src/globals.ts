import { CameraInfo } from "./camera/camera";
import { reactive } from "vue";
import { router } from "./router/routes";
import { replaceRoute } from "./router/helpers";
import { fetchOrErr, sleep, type FetchResult } from "./util/util";
import type { SystemInfoJSON, StartupErrorJSON } from "./api/api";
import { generateKeyPair } from "curve25519-js";
import { fetchAndValidateServerPublicKey } from "./auth";
import { natNotifyNetworkDown } from "./nativeOut";

// A global reactive object used throughout the app
export class Globals {
	// If isApp is true, then we are running in a WebView inside our mobile app.
	// If isApp is false, then we running in a regular browser (but could still be a mobile device).
	isApp = false;

	isUsingProxy = window.location.origin.startsWith("https://") && window.location.hostname.includes(".p.cyclopcam.org");

	isFirstVideoPlay = true;

	cameras: CameraInfo[] = [];

	objectClasses: string[] = []; // Objects detected by our neural network(s) (eg person, car, truck,...)
	abstractClasses: { [index: string]: string } = {}; // Abstract classes, eg {"car": "vehicle", "truck": "vehicle"}

	isLoggedIn = false; // Only valid after isSystemInfoLoadFinished = true
	startupErrors: StartupErrorJSON[] = []; // Only valid after isSystemInfoLoadFinished = true. If not empty, then host system needs configuring before it can start.
	isSystemInfoLoadFinished = false;

	// isServerPublicKeyLoaded is set to true after we've validated the server's
	// public key. Even if public key validation fails, we still set this to true.
	isServerPublicKeyLoaded = false;

	// Base64 encoding of public key of the server we're connected to.
	// We only set this key if the server proves that it owns this public key,
	// by signing a challenge that we send to it. If validation fails, then this
	// string remains empty.
	serverPublicKey = "";

	// The only reason we generate our own key pair is for the challenge/response
	// conversation with the server. If I knew how to use an X25519 key to
	// sign a message, then we wouldn't need this. i.e. We could get the server
	// to sign a challenge with an X25519 key. But anyway, this is FINE.
	ownPrivateKey: Uint8Array;
	ownPublicKey: Uint8Array;

	// JSON result of our last successful network scan for cameras.
	// This is a hack to preserve our most recent LAN scan, so that somebody can go through
	// a bunch of cameras, adding them one by one, without having to re-scan the LAN after
	// adding each camera. We can't store this data inside ScanForCameras.vue, because the
	// component will be recreated each time we navigate to it. I call this a hack because
	// this data ideally belongs somewhere else, but I can't think of a better place than
	// here.
	lastNetworkCameraScanJSON: any = null;

	// Password of the most recently added camera.
	// This is used to prefill the password when adding multiple cameras.
	lastCameraPassword = "";
	lastCameraUsername = ""; // Username of the most recently added camera.

	private _networkError = ""; // Most recent network error, typically shown in the top/bottom bar

	constructor() {
		console.log("globals constructor");
		let rnd = new Uint8Array(32);
		crypto.getRandomValues(rnd);
		let kp = generateKeyPair(rnd);
		this.ownPrivateKey = kp.private;
		this.ownPublicKey = kp.public;
	}

	// Most recent network error, typically shown in the top/bottom bar
	get networkError(): string {
		return this._networkError;
	}

	set networkError(value: string) {
		this._networkError = value;
		if (value && value.indexOf("<HEAD><TITLE>Connection timed out") !== -1) {
			// Android's WebView will send us this kind of nonsense if we were connected to our
			// server via LAN, and we lose Wefi. We need to inform the Java app that it needs
			// to switch to the proxy.
			natNotifyNetworkDown(value);
		}
	}

	// Wait for public key load to finish.
	async waitForPublicKeyLoad() {
		for (let i = 0; i < 300; i++) {
			if (this.isServerPublicKeyLoaded) {
				return;
			}
			await sleep(10);
		}
	}

	// Wait for system info load to finish
	async waitForSystemInfoLoad() {
		for (let i = 0; i < 300; i++) {
			if (this.isSystemInfoLoadFinished) {
				return;
			}
			await sleep(10);
		}
	}

	async loadPublicKey() {
		this.serverPublicKey = await fetchAndValidateServerPublicKey(this.ownPrivateKey, this.ownPublicKey);
		this.isServerPublicKeyLoaded = true;
	}

	async bootup(setVueRoute: boolean) {
		await this.loadPublicKey();

		try {
			let r = await fetch("/api/auth/whoami");
			if (r.status === 403) {
				this.isLoggedIn = false;
				r = await (await fetch("/api/auth/hasAdmin")).json();
				if (r) {
					replaceRoute(router, { name: "rtLogin" });
				} else {
					replaceRoute(router, { name: "rtWelcome" });
				}
			} else if (r.status === 200) {
				this.isLoggedIn = true;
				await this.postAuthenticateLoadSystemInfo(setVueRoute);
			}
		} catch (err) {
			console.log("bootup failed", err);
		}

		this.isSystemInfoLoadFinished = true;
	}

	async postAuthenticateLoadSystemInfo(setVueRoute: boolean) {
		let root = await (await fetch("/api/system/info")).json();
		this.startupErrors = root.startupErrors;
		if (this.startupErrors.length > 0)
			console.log(`startupErrors`, root.startupErrors);
		this.objectClasses = root.objectClasses;
		this.abstractClasses = root.abstractClasses;

		// I'd rather get rid of this special welcome screen and just use our regular
		// configuration screens, even on initial load.
		let gotoWelcome = root.cameras.length === 0;
		gotoWelcome = false;

		if (gotoWelcome) {
			if (setVueRoute)
				replaceRoute(router, { name: "rtWelcome" });
		} else {
			// This is supposed to be a catch-all place where we fetch data
			// about all cameras in the system, regardless of how the user
			// has navigated to our app.
			// It feels like there should be a better place than this...
			// perhaps on a 'preload' property of a top-level route or something.
			this.loadCamerasFromInfo(root);

			if (setVueRoute) {
				let current = router.currentRoute.value.name;
				if (!current || current === "rtHome" || current === "rtLogin") {
					replaceRoute(router, { name: "rtMonitor" });
				}
			}
		}
	}

	// Restart the server
	// Returns an empty string on success.
	async restart(timeoutSeconds: number): Promise<string> {
		let r = await fetchOrErr('/api/system/restart', { method: 'POST' });
		if (!r.ok) {
			return r.error;
		}
		let start = new Date().getTime();
		while (true) {
			await sleep(200);
			let r = await fetchOrErr('/api/system/info');
			if (r.ok)
				break;
			if (new Date().getTime() - start > timeoutSeconds * 1000)
				return "Timeout";
		}
		await this.postAuthenticateLoadSystemInfo(false);
		return "";
	}

	async loadCameras() {
		let res = await fetchOrErr("/api/system/info");
		if (!res.ok) {
			return;
		}
		let infj = (await res.r.json()) as SystemInfoJSON;
		this.loadCamerasFromInfo(infj);
	}

	loadCamerasFromInfo(infoJSON: any) {
		let cameras = [];
		for (let jc of infoJSON.cameras) {
			cameras.push(CameraInfo.fromJSON(jc));
		}
		this.cameras = cameras;
	}

	// Wrapper around util.fetchOrErr, which sets networkError on failure.
	async fetchOrErr(url: string, options?: RequestInit): Promise<FetchResult> {
		let r = await fetchOrErr(url, options);
		if (!r.ok) {
			this.networkError = r.error;
		}
		return r;
	}
}

export const globals = reactive(new Globals());


