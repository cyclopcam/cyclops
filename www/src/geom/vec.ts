export class Vec2 {
	x = 0;
	y = 0;

	constructor(x: number, y: number) {
		this.x = x;
		this.y = y;
	}

	static fromJson(j: any): Vec2 {
		return new Vec2(j[0], j[1]);
	}

	static lerp(a: Vec2, b: Vec2, r: number): Vec2 {
		return new Vec2(a.x + (b.x - a.x) * r, a.y + (b.y - a.y) * r);
	}

	static cosSin(theta: number, r = 1): Vec2 {
		return new Vec2(r * Math.cos(theta), r * Math.sin(theta));
	}

	static size(x: number, y: number): number {
		return Math.hypot(x, y);
	}

	get size(): number {
		return Math.hypot(this.x, this.y);
	}

	set(x: number, y: number) {
		this.x = x;
		this.y = y;
	}

	normalized(): Vec2 {
		let is = 1 / this.size;
		return new Vec2(this.x * is, this.y * is);
	}

	clone(): Vec2 {
		return new Vec2(this.x, this.y);
	}

	rounded(): Vec2 {
		return new Vec2(Math.round(this.x), Math.round(this.y));
	}

	distance(p: Vec2): number {
		return Math.hypot(p.x - this.x, p.y - this.y);
	}

	distanceSQ(p: Vec2): number {
		return (p.x - this.x) * (p.x - this.x) + (p.y - this.y) * (p.y - this.y);
	}

	distanceXY(x: number, y: number): number {
		return Math.hypot(x - this.x, y - this.y);
	}

	distanceXYSQ(x: number, y: number): number {
		return (x - this.x) * (x - this.x) + (y - this.y) * (y - this.y);
	}

	equals(b: Vec2): boolean {
		return this.x === b.x && this.y === b.y;
	}

	sub(b: Vec2): Vec2 {
		return new Vec2(this.x - b.x, this.y - b.y);
	}

	add(b: Vec2): Vec2 {
		return new Vec2(this.x + b.x, this.y + b.y);
	}

	mul(s: number): Vec2 {
		return new Vec2(this.x * s, this.y * s);
	}

	dot(b: Vec2): number {
		return this.x * b.x + this.y * b.y;
	}

	toString(precision = 3): string {
		return this.x.toFixed(precision) + "," + this.y.toFixed(precision);
	}
}

export class Vec2Pair {
	a: Vec2;
	b: Vec2;

	constructor(a: Vec2, b: Vec2) {
		this.a = a;
		this.b = b;
	}
}



export class Vec3 {
	x = 0;
	y = 0;
	z = 0;

	constructor(x: number, y: number, z: number) {
		this.x = x;
		this.y = y;
		this.z = z;
	}

	static fromJson(j: any): Vec3 {
		return new Vec3(j[0], j[1], j[2]);
	}

	clone(): Vec3 {
		return new Vec3(this.x, this.y, this.z);
	}

	set(x: number, y: number, z: number) {
		this.x = x;
		this.y = y;
		this.z = z;
	}

	dot(v: Vec3): number {
		return this.x * v.x + this.y * v.y + this.z * v.z;
	}

	dot2(v: Vec2, z = 1): number {
		return this.x * v.x + this.y * v.y + this.z * z;
	}

	dotXY(x: number, y: number, z = 1): number {
		return this.x * x + this.y * y + this.z * z;
	}
}