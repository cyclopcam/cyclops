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
