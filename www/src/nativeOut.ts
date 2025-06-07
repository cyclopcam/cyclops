// nativeOut has functions that we use to talk to our native (Java/Swift) component

import { encodeQuery } from "./util/util";

export enum OAuthLoginPurpose {
	InitialUser = "initialUser",
	Login = "login",
}

// We call this after a successful login to a server
export async function natLogin(publicKey: string, bearerToken: string, sessionCookie: string) {
	await fetch("/natcom/login?" + encodeQuery({ publicKey, bearerToken, sessionCookie }));
}

export async function natPostLogin() {
	await fetch("/natcom/postLogin");
}

export async function natNotifyNetworkDown(errorMsg: string) {
	await fetch("/natcom/networkDown?" + encodeQuery({ errorMsg }));
}

// Called by the welcome screen (and perhaps the login screen), when the user clicks "Login with Google", "Login with Microsoft" etc.
// For the login screen, we should be doing this transparently, without needing to show the login screen.
// But this function was created for the welcome screen, when creating the initial user.
export async function natRequestOAuthLogin(purpose: OAuthLoginPurpose, provider: string) {
	console.log("natRequestOAuthLogin");
	await fetch("/natcom/requestOAuthLogin?" + encodeQuery({ provider, purpose }));
}

export type NativeDecoderID = string;

// Create a video decoder.
// You must destroy it when you're done with it.
// Returns the ID of the decoder.
export async function natCreateVideoDecoder(codec: string): Promise<NativeDecoderID> {
	let r = await fetch("/natcom/decoder/create?" + encodeQuery({ codec }));
	return await r.text() as NativeDecoderID;
}

// Destroy a video decoder
export async function natDestroyVideoDecoder(decoderId: NativeDecoderID) {
	await fetch("/natcom/decoder/destroy?" + encodeQuery({ decoderId }));
}

// Decode a video packet.
export async function natDecodeVideoPacket(decoderId: NativeDecoderID, packet: Uint8Array) {
	await fetch("/natcom/decoder/packet?" + encodeQuery({ decoderId }), {
		method: "POST",
		body: packet,
	});
}

// Extract the next frame from a video decoder.
export async function natNextVideoFrame(decoderId: NativeDecoderID): Promise<ImageBitmap | null> {
	let r = await fetch("/natcom/decoder/nextFrame?" + encodeQuery({ decoderId }));
	if (r.status === 204) {
		// No content, no frame available.
		return null;
	}
	if (r.status !== 200) {
		console.error("Failed to get next video frame:", r.status, r.statusText);
		return null;
	}
	let blob = await r.blob();
	return createImageBitmap(blob);
}
