// nativeOut has functions that we use to talk to our native (Java/Swift) component

import { globals } from "./globals";
import { encodeQuery } from "./util/util";

export async function natLogin(publicKey: string, bearerToken: string, sessionCookie: string) {
	await fetch("/natcom/login?" + encodeQuery({ publicKey, bearerToken, sessionCookie }));
}

// Returns an empty string on success, or an error
// DELETE ME
export async function natLogin2(publicKey: string, username: string, password: string): Promise<string> {
	let resp = await fetch("/natcom/login2?" + encodeQuery({ publicKey, username, password }));
	if (!resp.ok) {
		return resp.text();
	}
	globals.isLoggedIn = true;
	return "";
}