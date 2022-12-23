import { CameraInfo } from "./camera/camera";
import { reactive } from "vue";
import { replaceRoute, router } from "./router/routes";
import { fetchOrErr, sleep, type FetchResult } from "./util/util";
import type { SystemInfoJSON } from "./api/api";
import { generateKeyPair, sharedKey } from "curve25519-js";
import * as base64 from "base64-arraybuffer";
import { fetchAndValidateServerPublicKey, getBearerToken } from "./auth";
import type { Chacha20 } from "ts-chacha20";

// A global reactive object used throughout the app
export class Globals {
	// If isApp is true, then we are running in a WebView inside our mobile app.
	// If isApp is false, then we running in a regular browser.
	isApp = false;

	isUsingProxy = window.location.origin.startsWith("https://proxy");

	isFirstVideoPlay = true;

	cameras: CameraInfo[] = [];
	networkError = ""; // Most recent network error, typically shown in the top/bottom bar
	isLoggedIn = false; // Only valid after isSystemInfoLoadFinished = true
	readyError = ""; // Only valid after isSystemInfoLoadFinished = true. If not empty, then host system needs configuring before it can start.
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
	// sign a message, then we wouldn't need this.
	ownPrivateKey: Uint8Array;
	ownPublicKey: Uint8Array;

	//ownPublicKeyBase64: string;
	//sharedTokenKey: Uint8Array | null = null;
	//sharedNonce: Uint8Array;
	//sharedNonceBase64: string;
	//sharedChaCha20: Chacha20 | null = null;
	//encryptedBearerToken = "";

	constructor() {
		console.log("globals constructor");
		let rnd = new Uint8Array(32);
		crypto.getRandomValues(rnd);
		let kp = generateKeyPair(rnd);
		this.ownPrivateKey = kp.private;
		this.ownPublicKey = kp.public;
		//this.ownPublicKeyBase64 = base64.encode(this.ownPublicKey);
		//this.sharedNonce = new Uint8Array(12);
		//crypto.getRandomValues(this.sharedNonce);
		//this.sharedNonceBase64 = base64.encode(this.sharedNonce);
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

		//try {
		//	let r = await fetch("/api/keys");
		//	if (r.status === 200) {
		//		let j = await r.json();
		//		//this.serverPublicKey = j.publicKey;
		//		//this.loadAndEncryptBearerToken();
		//	}
		//} catch (err) {
		//}
	}

	//loadAndEncryptBearerToken() {
	//	let serverKey = new Uint8Array(base64.decode(this.serverPublicKey));
	//	this.encryptedBearerToken = loadAndEncryptBearerToken(this.ownPrivateKey, serverKey, this.sharedNonce);
	//}

	async loadSystemInfo() {
		await this.loadPublicKey();

		try {
			let r = await fetch("/api/auth/whoami");
			// I've decided to move this code into the native app, because it's just so dangerous
			// to store these long term tokens in localStorage. For example, a malicious app on the
			// same Lan IP would be able to read from localStorage.. thereby exfiltrating all of your
			// bearer tokens.
			/*
			if (r.status === 403) {
				// try using our bearer token to login
				// TODO: Use private key instead of bearer token for this purpose
				if (getBearerToken() !== "") {
					console.log("Attemping to acquire new cookie");
					let rLogin = await fetchWithAuth("/api/auth/login?loginMode=Cookie", { method: "POST" });
					if (rLogin.status === 200) {
						// try again
						r = await fetch("/api/auth/whoami");
						console.log("whoami after cookie acquisition", r.status, r.statusText);
					}
				}
			}
			*/
			if (r.status === 403) {
				this.isLoggedIn = false;
				r = await (await fetch("/api/auth/hasAdmin")).json();
				if (r) {
					replaceRoute({ name: "rtLogin" });
				} else {
					replaceRoute({ name: "rtWelcome" });
				}
			} else if (r.status === 200) {
				this.isLoggedIn = true;
				await this.postLoadAutoRoute();
			}
		} catch (err) {
			console.log("loadSystemInfo failed", err);
		}

		this.isSystemInfoLoadFinished = true;
	}

	async postLoadAutoRoute() {
		let root = await (await fetch("/api/system/info")).json();
		if (root.readyError) {
			this.readyError = root.readyError;
			return;
		}

		if (root.cameras.length === 0) {
			replaceRoute({ name: "rtWelcome" });
		} else {
			// This is supposed to be a catch-all place where we fetch data
			// about all cameras in the system, regardless of how the user
			// has navigated to our app.
			// It feels like there should be a better place than this...
			// perhaps on a 'preload' property of a top-level route or something.
			this.loadCamerasFromInfo(root);

			let current = router.currentRoute.value.name;
			if (!current || current === "rtHome" || current === "rtLogin") {
				replaceRoute({ name: "rtMonitor" });
			}
		}
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

