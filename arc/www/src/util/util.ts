
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

export function parseUrlHash(): { [key: string]: string } {
	let hash = window.location.hash;
	if (hash.length === 0) {
		return {};
	}
	if (hash[0] === "#") {
		hash = hash.substring(1);
	}
	let kv: { [key: string]: string } = {};
	let parts = hash.split("&");
	for (let part of parts) {
		let kvParts = part.split("=");
		if (kvParts.length === 2) {
			kv[decodeURIComponent(kvParts[0])] = decodeURIComponent(kvParts[1]);
		}
	}
	return kv;
}

export type ErrorResponse = { ok: false; err: string };
export type SuccessResponse<T> = { ok: true; value: T };
export type OrError<T> = ErrorResponse | SuccessResponse<T>;

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
