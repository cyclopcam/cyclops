/*
export class FetchResult {
	status: number; // If status is zero, then the fetch threw an exception
	error?: string; // If error is defined, then the HTTP status code was not 200
	r?: Response; // The response is undefined if an exception was thrown

	constructor(status: number, r: Response | undefined, error: string | undefined) {
		this.status = status;
		this.r = r;
		this.error = error;
	}

	// Returns true if status === 200
	get ok(): boolean {
		return this.status === 200;
	}
}
*/

import { getBearerToken } from "@/auth";
import { globals } from "@/globals";

export type FetchSuccess = {
	ok: true;
	r: Response;
};

export type FetchFailure = {
	ok: false;
	status: number; // if no response, then status = 0
	error: string;
	r?: Response;
};

export type FetchResult = FetchSuccess | FetchFailure;

// fetchOrErr wraps common fetch failures into a unified object, so that
// you don't need to handle try/catch failures, as well as all the other types.
export async function fetchOrErr(url: string, options?: RequestInit): Promise<FetchResult> {
	try {
		let r = await fetch(url, options);
		if (r.status === 200) {
			return { ok: true, r };
		} else {
			if (r.body) {
				let txt = await r.text();
				return { ok: false, status: r.status, error: txt, r: r };
			} else {
				return { ok: false, status: r.status, error: r.statusText, r: r };
			}
		}
	} catch (err) {
		if (err + "" === "TypeError: Failed to fetch") {
			err = "Network error";
		}
		return { ok: false, status: 0, error: err + "" };
	}
}

export async function fetchWithAuth(input: RequestInfo | URL, init?: RequestInit | undefined): Promise<Response> {
	if (!init) {
		init = {};
	}
	if (!init.headers) {
		init.headers = {};
	}
	let bearerToken = getBearerToken();
	// If Authorization header is not present, then add our localStorage bearer token
	if (bearerToken) {
		let keys = Object.keys(init.headers);
		let hasAuthorization = false;
		for (let k of keys) {
			if (k === "Authorization") {
				hasAuthorization = true;
			}
		}
		if (!hasAuthorization) {
			init.headers = Object.assign(init.headers, {
				'Authorization': 'Bearer ' + bearerToken,
				//'X-PublicKey': globals.ownPublicKeyBase64,
				//'X-Nonce': globals.sharedNonceBase64,
			}
			);
		}
	}
	return fetch(input, init);
}

export function encodeQuery(kv: { [key: string]: string | number | boolean }): string {
	let s = "";
	for (let k in kv) {
		s += encodeURIComponent(k) + "=" + encodeURIComponent(kv[k]);
		s += "&";
	}
	if (s.length === 0) {
		return s;
	}
	return s.substring(0, s.length - 1);
}

export type ErrorResponse = { ok: false; err: string };
export type SuccessResponse<T> = { ok: true; value: T };
export type OrError<T> = ErrorResponse | SuccessResponse<T>;

/*
export class OrError<T> {
	err?: string;
	value?: T;

	constructor(value: T | undefined, error: string | undefined) {
		this.value = value;
		this.err = error;
	}

	static error<T>(err: string): OrError<T> {
		return new OrError<T>(undefined, err);
	}

	static value<T>(t: T): OrError<T> {
		return new OrError<T>(t, undefined);
	}
}
*/

export async function sleep(milliseconds: number): Promise<void> {
	return new Promise((resolve) => {
		setTimeout(resolve, milliseconds);
	});
}

export function randomString(length: number): string {
	let result = "";
	if (length === 0) return result;
	let letters = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz";
	let all = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789";

	// CSS DOM selectors (eg #xyz) are not allowed to start with a digit, so we ensure that
	// our first character is a letter.
	result += letters.charAt(Math.floor(Math.random() * letters.length));

	let allLen = all.length;
	for (let i = result.length; i < length; i++) {
		result += all.charAt(Math.floor(Math.random() * allLen));
	}
	return result;
}

export const monthNames = ["January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"];

export function zeroPad(v: number, digits: number): string {
	let s = Math.round(v).toString();
	while (s.length < digits) {
		s = "0" + s;
	}
	return s;
}

export function dateTime(t: Date): string {
	return `${t.getFullYear()} ${monthNames[t.getMonth()]} ${t.getDay()} ${zeroPad(t.getHours(), 2)}:${zeroPad(t.getMinutes(), 2)}`;
}

export function dateTimeShort(t: Date): string {
	return `${t.getFullYear()}/${t.getMonth() + 1}/${t.getDay()} ${zeroPad(t.getHours(), 2)}:${zeroPad(t.getMinutes(), 2)}`;
}

export function clamp(v: number, min: number, max: number): number {
	if (v < min) {
		return min;
	} else if (v > max) {
		return max;
	}
	return v;
}
