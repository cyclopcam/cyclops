export class Globals {

	async redirectIfNotLoggedIn() {
		try {
			let r = await fetch('/api/auth/check');
			if (r.status === 200) {
				return;
			} else if (r.status === 401) {
				window.location.href = "/login";
				return;
			}
		} catch (e) {
			console.error(e);
		}
	}
}

export const globals = new Globals();