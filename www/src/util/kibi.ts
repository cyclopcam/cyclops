export function formatByteSize(b: number): string {
	if (b < 1024)
		return `${b} bytes`;
	if (b < 1024 * 1024)
		return `${(b / 1024).toFixed(0)} KB`;
	if (b < 1024 * 1024 * 1024)
		return `${(b / (1024 * 1024)).toFixed(0)} MB`;
	if (b < 1024 * 1024 * 1024 * 1024)
		return `${(b / (1024 * 1024 * 1024)).toFixed(0)} GB`;

	return `${(b / (1024 * 1024 * 1024 * 1024)).toFixed(0)} TB`;
}

// Split '500 MB' into { value: 500, unit: 'MB' }
export function kibiSplit(s: string): { value: number, unit: string } {
	if (s.length === 0) {
		return { value: 0, unit: "" };
	}
	let value = parseInt(s);
	let unit = s.substring(value.toString().length).toUpperCase().trim();
	return { value, unit };
}