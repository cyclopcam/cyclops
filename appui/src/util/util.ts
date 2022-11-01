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

export async function sleep(milliseconds: number): Promise<void> {
	return new Promise((resolve) => {
		setTimeout(resolve, milliseconds);
	});
}
