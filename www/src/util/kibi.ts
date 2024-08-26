export type ByteSizeUnit = 'bytes' | 'KB' | 'MB' | 'GB' | 'TB';

export function byteSizeUnit(b: number): ByteSizeUnit {
	if (b < 1024)
		return `bytes`;
	if (b < 1024 * 1024)
		return `KB`;
	if (b < 1024 * 1024 * 1024)
		return `MB`;
	if (b < 1024 * 1024 * 1024 * 1024)
		return `GB`;
	return `TB`;
}

export function formatByteSize(b: number, unit?: ByteSizeUnit, includeUnit = true): string {
	if (!unit) {
		unit = byteSizeUnit(b);
	}
	let div = 1
	switch (unit) {
		case 'bytes': div = 1; break;
		case 'KB': div = 1024; break;
		case 'MB': div = 1024 * 1024; break;
		case 'GB': div = 1024 * 1024 * 1024; break;
		case 'TB': div = 1024 * 1024 * 1024 * 1024; break;
	}
	if (includeUnit)
		return `${(b / div).toFixed(0)} ${unit}`;
	else
		return `${(b / div).toFixed(0)}`;
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