// Return the intersection of two spans [a1, a2] and [b1, b2].
// If the spans do not intersect, return [0, 0].
export function spanIntersection(a1: number, a2: number, b1: number, b2: number): [number, number] {
	if (a1 > a2) {
		[a1, a2] = [a2, a1];
	}
	if (b1 > b2) {
		[b1, b2] = [b2, b1];
	}
	if (a1 > b2 || b1 > a2) {
		return [0, 0];
	}
	return [Math.max(a1, b1), Math.min(a2, b2)];
}