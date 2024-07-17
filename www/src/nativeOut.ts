// nativeOut has functions that we use to talk to our native (Java/Swift) component

import { encodeQuery } from "./util/util";

export async function natLogin(publicKey: string, bearerToken: string, sessionCookie: string) {
	await fetch("/natcom/login?" + encodeQuery({ publicKey, bearerToken, sessionCookie }));
}

export async function natNotifyNetworkDown(errorMsg: string) {
	await fetch("/natcom/networkDown?" + encodeQuery({ errorMsg }));
}
