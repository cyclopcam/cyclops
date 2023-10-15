export class Globals {
	// Right now we're making all the training videos public, but we might want to
	// offer private videos in future. In that case, you would fetch the video content
	// either through a signed GCS/S3 URL, or via the arc server.
	// If publicVideoBaseUrl is empty, then it means the videos are private.
	publicVideoBaseUrl = ''; // eg 

	async fetchConstantsAndRedirectIfNotLoggedIn() {
		try {
			let r = await fetch('/api/constants');
			if (r.status === 200) {
				let j = await r.json();
				this.publicVideoBaseUrl = j.publicVideoBaseUrl;
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