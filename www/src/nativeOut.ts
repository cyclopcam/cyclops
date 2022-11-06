// nativeOut has functions that we use to talk to our native (Java/Swift) component

import { globals } from "./globals";
import { encodeQuery } from "./util/util";

export async function natLogin(publicKey: string, bearerToken: string) {
	await fetch("/natcom/login?" + encodeQuery({ publicKey: publicKey, bearerToken: bearerToken }));
}

// Returns an empty string on success, or an error
export async function natLogin2(publicKey: string, username: string, password: string): Promise<string> {
	let resp = await fetch("/natcom/login2?" + encodeQuery({ publicKey: publicKey, username: username, password: password }));
	if (!resp.ok) {
		return resp.text();
	}
	globals.isLoggedIn = true;
	return "";
}