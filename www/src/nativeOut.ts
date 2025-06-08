// nativeOut has functions that we use to talk to our native (Java/Swift) component

import { encodeQuery, fetchOrErr } from "./util/util";

export enum OAuthLoginPurpose {
	InitialUser = "initialUser",
	Login = "login",
}

// And Android WebView, if I send a 400, then the body is not transmitted to us.
// So we have this hack where text messages are prefixed with "ERROR:" to indicate an error.
function handleNatError(txt: string): string {
	if (txt.startsWith("ERROR:")) {
		let msg = txt.substring(6);
		throw new Error(msg);
	}
	return txt;
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

export async function natWsVideoPlay(wsUrl: string, codec: string, width: number, height: number): Promise<NativeDecoderID> {
	let r = await fetch("/natcom/wsvideo/play?" + encodeQuery({ wsurl: wsUrl, codec, width, height }));
	let txt = handleNatError(await r.text());
	return txt as NativeDecoderID;
}

export async function natWsVideoStop(id: NativeDecoderID) {
	return fetch("/natcom/wsvideo/stop?" + encodeQuery({ id }));
}

export async function natWsVideoNextFrame(id: NativeDecoderID, width: number, height: number): Promise<ImageBitmap | null> {
	let r = await fetch("/natcom/wsvideo/nextframe?" + encodeQuery({ id }));
	//console.log(`natWsVideoNextFrame: ${r.status} ${r.statusText}`);
	if (r.status === 204) {
		// No content, no frame available.
		return null;
	} else if (r.status === 200) {

		// --- convert raw RGBA to ImageBitmap ------------------------------
		let buf = await r.arrayBuffer();                 // raw RGBA
		if (!buf.byteLength) {
			console.log(`natWsVideoNextFrame: byteLength is 0`);
			return null;
		}

		//let { width, height } = getDimensions(id);       // <- your stored values
		// (width * height * 4  === buf.byteLength should always hold)

		let pixels = new Uint8ClampedArray(buf);         // RGBA8 → JS view
		let imageData = new ImageData(pixels, width, height);

		//console.log(`natWsVideoNextFrame: A REAL FRAME!`);

		// Turn it into a GPU-backed bitmap.  No premultiplication or colour conversion needed.
		return await createImageBitmap(imageData, {
			premultiplyAlpha: "none",
			colorSpaceConversion: "none",
		});
	}
	return null;
}

// Create a video decoder.
// You must destroy it when you're done with it.
// Returns the ID of the decoder.
export async function natCreateVideoDecoder(codec: string, width: number, height: number): Promise<NativeDecoderID> {
	let r = await fetch("/natcom/decoder/create?" + encodeQuery({ codec, width, height }));
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
