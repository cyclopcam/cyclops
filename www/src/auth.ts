import { globals } from "./globals";
import { encodeQuery, fetchOrErr } from "./util/util";
import { sharedKey } from "curve25519-js";
import { sha256 } from "js-sha256";
import * as base64 from "base64-arraybuffer";
import { Chacha20 } from "ts-chacha20";
import { hmac256_sign } from "./util/hmac";
//import { createHmac } from "crypto";

// If we are logged in to a server with a public wireguard key, then return the session
// token for that server, encrypted with the key pair (Server, Ours).
export function getBearerToken(): string {
	//console.log("getBearerToken");
	//return globals.encryptedBearerToken;
	//return globals.bearerToken;
	//if (!globals.sharedChaCha20) {
	//	return "";
	//}
	let publicKey = globals.serverPublicKey;
	if (publicKey === "") {
		return "";
	}
	let token = localStorage.getItem(publicKey + "-token");
	if (!token) {
		return "";
	}
	return token;
	/*

	//let tokenRaw = new Uint8Array(base64.decode(token));
	//return base64.encode(globals.sharedChaCha20.encrypt(tokenRaw));
	return globals.encryptedBearerToken;
	*/

	//globals.sharedChaCha20.encrypt()
	/*
	// Return SHA256(SharedSecret || Token)
	let rawMsg = new Uint8Array(globals.sharedTokenKey.length + token.length);
	rawMsg.set(globals.sharedTokenKey, 0);
	let j = globals.sharedTokenKey.length;
	for (let i = 0; i < token.length; i++) {
		rawMsg[j++] = token.charCodeAt(i);
	}
	return sha256(rawMsg);
	*/
}

/*
export function loadAndEncryptBearerToken(ownPrivateKey: Uint8Array, serverPublicKey: Uint8Array, nonce: Uint8Array): string {
	let server64 = base64.encode(serverPublicKey);
	let token = localStorage.getItem(server64 + "-token");
	if (!token) {
		return "";
	}
	let tokenRaw = new Uint8Array(base64.decode(token));
	return encryptBearerToken(ownPrivateKey, serverPublicKey, nonce, tokenRaw);
}

export function encryptBearerToken(ownPrivateKey: Uint8Array, serverPublicKey: Uint8Array, nonce: Uint8Array, token: Uint8Array): string {
	let shared = createSharedSecretKey(ownPrivateKey, serverPublicKey);
	let chacha = new Chacha20(shared, nonce, 0);
	return base64.encode(chacha.encrypt(token));
}
*/

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

export function setBearerToken(publicKey: string, tokenb64: string) {
	localStorage.setItem(publicKey + "-token", tokenb64);
}

// Returns an error on failure, or an empty string on success
export async function login(username: string, password: string): Promise<string> {
	let getToken = globals.serverPublicKey !== '';

	console.log("Logging in with CookieAndBearerToken");
	let basic = btoa(username.trim() + ":" + password.trim());
	//let r = await fetchOrErr('/api/auth/login?' + encodeQuery({ loginMode: getToken ? "BearerToken" : "Cookie" }),
	let r = await fetchOrErr('/api/auth/login?' + encodeQuery({ loginMode: "CookieAndBearerToken" }),
		{ method: 'POST', headers: { "Authorization": "BASIC " + basic } });
	if (!r.ok) {
		return r.error;
	}
	globals.isLoggedIn = true;

	let j = await r.r.json();

	if (getToken) {
		let tokenb64 = j.bearerToken;
		setBearerToken(globals.serverPublicKey, tokenb64);
		// Inform our mobile app that we've logged in. Chrome's limit on cookie duration is about 400 days,
		// but we can extend that by not using cookies. Also, the mobile app needs to know the list of
		// servers that the client knows about.
		fetch('/natcom/login?' + encodeQuery({ publicKey: globals.serverPublicKey, bearerToken: tokenb64 }));
	} else {
		// In this case, at least the mobile app knows to store this server IP address
		fetch('/natcom/login');
	}
	return "";
}

export function createSharedSecretKey(privateKey: Uint8Array, publicKey: Uint8Array): Uint8Array {
	let raw = sharedKey(privateKey, publicKey);
	return raw;
	//return new Uint8Array(sha256.arrayBuffer(raw));
}

export function createSharedSecretChaCha20(privateKey: Uint8Array, publicKey: Uint8Array, nonce: Uint8Array): Chacha20 {
	let raw = sharedKey(privateKey, publicKey);
	return new Chacha20(raw, nonce, 0);
}

export function bearerTokenQuery(): { authorizationToken: string } | {} {
	let token = getBearerToken();
	if (token) {
		return { authorizationToken: token };
	} else {
		return {};
	}
}