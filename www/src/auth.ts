import { globals } from "./globals";
import { encodeQuery, fetchOrErr, type FetchResult, type FetchSuccess } from "./util/util";
import { sharedKey } from "curve25519-js";
import { sha256 } from "js-sha256";
import * as base64 from "base64-arraybuffer";
import { Chacha20 } from "ts-chacha20";
import { hmac256_sign } from "./util/hmac";
import { natLogin } from "./nativeOut";
//import { createHmac } from "crypto";

// SYNC-LOGIN-RESPONSE-JSON
interface LoginResponse {
	bearerToken: string;
	needRestart: boolean;
}

interface LoginSuccess {
	ok: true;
	response: LoginResponse;
}

interface LoginFail {
	ok: false;
	error: string;
}

type LoginResult = LoginSuccess | LoginFail;

// If we are logged in to a server with a public wireguard key, then return the session
// token for that server, encrypted with the key pair (Server, Ours).
export function getBearerToken(): string {
	let publicKey = globals.serverPublicKey;
	if (publicKey === "") {
		return "";
	}
	let token = localStorage.getItem(publicKey + "-token");
	if (!token) {
		return "";
	}
	return token;
}

// Fetches the server's public key and validates it.
// If validation passes, then returns the base64 encoded server's public key.
// In case of error, returns an empty string.
export async function fetchAndValidateServerPublicKey(ownPrivateKey: Uint8Array, ownPublicKey: Uint8Array): Promise<string> {
	try {
		let challenge = new Uint8Array(32);
		crypto.getRandomValues(challenge);

		let r = await fetch("/api/keys?" + encodeQuery({ publicKey: base64.encode(ownPublicKey), challenge: base64.encode(challenge) }));
		if (r.status === 200) {
			let j = await r.json();
			let serverPublicKey = new Uint8Array(base64.decode(j.publicKey));
			let serverProof = new Uint8Array(base64.decode(j.proof));
			let shared = sharedKey(ownPrivateKey, serverPublicKey);
			let digest = hmac256_sign(shared, challenge);
			if (areBuffersEqual(digest, serverProof)) {
				console.info(`Server public key ${j.publicKey} verified`);
				return j.publicKey;
			} else {
				console.warn(`Failed to validate server public key: digest does not match`);
			}
		}
	} catch (err) {
		console.warn(`Failed to validate server public key: ${err}`);
	}
	return "";
}

function areBuffersEqual(a: Uint8Array, b: Uint8Array): boolean {
	if (a.length !== b.length) {
		return false;
	}
	let x = 0;
	for (let i = 0; i < a.length; i++) {
		x |= a[i] ^ b[i];
	}
	return x === 0;
}

// Returns an error on failure, or an empty string on success
export async function login(username: string, password: string): Promise<LoginResult> {
	if (globals.serverPublicKey === '') {
		return { ok: false, error: "Server failed to validate its public key" };
	}

	// For a while, I started down the road of having logins be a two step process:
	// 1. Get the native app to login with a long term bearer token
	// 2. Get the native app to use that bearer token to login with a cookie
	// However, that just makes for more native code.
	// We are capable of doing 1 & 2 in a single step here, so we might as well.

	let loginMode = "Cookie";
	if (globals.isApp) {
		loginMode = "CookieAndBearerToken";
	}

	console.log(`Logging in with ${loginMode}`);
	let basic = btoa(username.trim() + ":" + password.trim());
	let r = await fetchOrErr('/api/auth/login?' + encodeQuery({ loginMode: loginMode }),
		{ method: 'POST', headers: { "Authorization": "BASIC " + basic } });
	if (!r.ok) {
		console.log(`Login error: ${r.error}`);
		return { ok: false, error: r.error };
	}

	return handleLoginSuccess(r);
}

// Returns a non-empty string on error, or "" on success
export async function handleLoginSuccess(r: FetchSuccess): Promise<LoginResult> {
	globals.isLoggedIn = true;

	let j = await r.r.json() as LoginResponse;

	if (globals.isApp) {
		console.log(`Sending tokens to app`);

		let bearerToken = j.bearerToken; // base64-encoded bearer token
		//setBearerToken(globals.serverPublicKey, bearerToken);
		// Inform our mobile app that we've logged in. Chrome's limit on cookie duration is about 400 days,
		// but we can extend that by not using cookies. Also, the mobile app needs to know the list of
		// servers that the client knows about.

		// Get our 'session' cookie
		let sessionCookie = getCookie("session");
		if (!sessionCookie) {
			return { ok: false, error: "Failed to get session cookie" };
		}
		console.log(`Login to ${globals.serverPublicKey} with session ${sessionCookie} and bearer token ${bearerToken}`);

		natLogin(globals.serverPublicKey, bearerToken, sessionCookie);
	} else {
		console.log(`App mode not activated, so not sending tokens to app`);
	}
	return { ok: true, response: j };
}

// Returns the cookie with the given name, or undefined if not found
// Source: https://javascript.info/cookie
function getCookie(name: string) {
	let matches = document.cookie.match(new RegExp(
		"(?:^|; )" + name.replace(/([\.$?*|{}\(\)\[\]\\\/\+^])/g, '\\$1') + "=([^;]*)"
	));
	return matches ? decodeURIComponent(matches[1]) : undefined;
}
