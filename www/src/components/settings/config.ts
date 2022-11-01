export let recentUsernames: string[] = [];
export let recentPasswords: string[] = [];

export function addToRecentUsernames(u: string) {
	if (!recentUsernames.includes(u)) recentUsernames.push(u);
}

export function addToRecentPasswords(u: string) {
	if (!recentPasswords.includes(u)) recentPasswords.push(u);
}

export interface CameraTestResult {
	error?: string;
	image?: Blob;
}
