import type { Vec2 } from "@/geom/vec";


export class Rect {
	x1: number;
	y1: number;
	x2: number;
	y2: number;

	constructor(x1: number, y1: number, x2: number, y2: number) {
		this.x1 = x1;
		this.y1 = y1;
		this.x2 = x2;
		this.y2 = y2;
	}

	static inverted(): Rect {
		return new Rect(9e30, 9e30, -9e30, -9e30);
	}

	expandToFitPt(pt: Vec2) {
		this.x1 = Math.min(this.x1, pt.x);
		this.y1 = Math.min(this.y1, pt.y);
		this.x2 = Math.max(this.x2, pt.x);
		this.y2 = Math.max(this.y2, pt.y);
	}

	get width(): number {
		return this.x2 - this.x1;
	}

	get height(): number {
		return this.y2 - this.y1;
	}
}