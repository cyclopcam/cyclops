import { Vec2 } from './vec';

export class Mat2 {
	a = new Vec2(1, 0);
	b = new Vec2(0, 1);

	mul(v: Vec2): Vec2 {
		return new Vec2(v.dot(this.a), v.dot(this.b));
	}

	static rotation(angleRadians: number): Mat2 {
		let m = new Mat2();
		m.a.x = Math.cos(angleRadians);
		m.a.y = -Math.sin(angleRadians);
		m.b.x = -m.a.y;
		m.b.y = m.a.x;
		return m;
	}
}

export class Mat3 {
	m: Float64Array;

	constructor(initialValue?: Float64Array) {
		if (initialValue) {
			this.m = new Float64Array(initialValue);
		} else {
			this.m = new Float64Array(9);
			this.setIdentity();
		}
	}

	clone(): Mat3 {
		return new Mat3(this.m);
	}

	/* eslint-disable */
	get v00(): number {
		return this.m[0];
	}
	get v01(): number {
		return this.m[1];
	}
	get v02(): number {
		return this.m[2];
	}
	get v10(): number {
		return this.m[3];
	}
	get v11(): number {
		return this.m[4];
	}
	get v12(): number {
		return this.m[5];
	}
	get v20(): number {
		return this.m[6];
	}
	get v21(): number {
		return this.m[7];
	}
	get v22(): number {
		return this.m[8];
	}
	set v00(v: number) {
		this.m[0] = v;
	}
	set v01(v: number) {
		this.m[1] = v;
	}
	set v02(v: number) {
		this.m[2] = v;
	}
	set v10(v: number) {
		this.m[3] = v;
	}
	set v11(v: number) {
		this.m[4] = v;
	}
	set v12(v: number) {
		this.m[5] = v;
	}
	set v20(v: number) {
		this.m[6] = v;
	}
	set v21(v: number) {
		this.m[7] = v;
	}
	set v22(v: number) {
		this.m[8] = v;
	}

	get XX(): number {
		return this.m[0];
	}
	get XY(): number {
		return this.m[1];
	}
	get XZ(): number {
		return this.m[2];
	}
	get YX(): number {
		return this.m[3];
	}
	get YY(): number {
		return this.m[4];
	}
	get YZ(): number {
		return this.m[5];
	}
	get ZX(): number {
		return this.m[6];
	}
	get ZY(): number {
		return this.m[7];
	}
	get ZZ(): number {
		return this.m[8];
	}
	set XX(v: number) {
		this.m[0] = v;
	}
	set XY(v: number) {
		this.m[1] = v;
	}
	set XZ(v: number) {
		this.m[2] = v;
	}
	set YX(v: number) {
		this.m[3] = v;
	}
	set YY(v: number) {
		this.m[4] = v;
	}
	set YZ(v: number) {
		this.m[5] = v;
	}
	set ZX(v: number) {
		this.m[6] = v;
	}
	set ZY(v: number) {
		this.m[7] = v;
	}
	set ZZ(v: number) {
		this.m[8] = v;
	}

	/* eslint-enable */

	setIdentity() {
		this.m.fill(0);
		this.v00 = 1;
		this.v11 = 1;
		this.v22 = 1;
	}

	mulXY(x: number, y: number, z = 1): Vec2 {
		return new Vec2(this.v00 * x + this.v01 * y + this.v02 * z, this.v10 * x + this.v11 * y + this.v12 * z);
	}

	v(row: number, col: number): number {
		return this.m[row * 3 + col];
	}

	set(row: number, col: number, val: number) {
		this.m[row * 3 + col] = val;
	}

	inverted(): Mat3 {
		let b = new Mat3();
		const det = this.v00 * (this.v11 * this.v22 - this.v12 * this.v21) -
			this.v01 * (this.v10 * this.v22 - this.v12 * this.v20) +
			this.v02 * (this.v10 * this.v21 - this.v11 * this.v20);

		if (Math.abs(det) < 1e-20)
			return b;

		const idet = 1.0 / det;

		b.v00 = idet * (this.v11 * this.v22 - this.v12 * this.v21);
		b.v01 = -idet * (this.v01 * this.v22 - this.v02 * this.v21);
		b.v02 = idet * (this.v01 * this.v12 - this.v02 * this.v11);

		b.v10 = -idet * (this.v10 * this.v22 - this.v12 * this.v20);
		b.v11 = idet * (this.v00 * this.v22 - this.v02 * this.v20);
		b.v12 = -idet * (this.v00 * this.v12 - this.v02 * this.v10);

		b.v20 = idet * (this.v10 * this.v21 - this.v11 * this.v20);
		b.v21 = -idet * (this.v00 * this.v21 - this.v01 * this.v20);
		b.v22 = idet * (this.v00 * this.v11 - this.v01 * this.v10);

		return b;
	}

	postmultiply(m: Mat3) {
		const oxy = this.XY;
		const oyx = this.YX;
		const oyz = this.YZ;
		const ozy = this.ZY;
		const ozx = this.ZX;
		const oxz = this.XZ;

		this.XY = this.XX * m.XY + oxy * m.YY + oxz * m.ZY;
		this.XZ = this.XX * m.XZ + oxy * m.YZ + oxz * m.ZZ;
		this.YX = oyx * m.XX + this.YY * m.YX + oyz * m.ZX;
		this.YZ = oyx * m.XZ + this.YY * m.YZ + oyz * m.ZZ;
		this.ZX = ozx * m.XX + ozy * m.YX + this.ZZ * m.ZX;
		this.ZY = ozx * m.XY + ozy * m.YY + this.ZZ * m.ZY;

		this.XX = this.XX * m.XX + oxy * m.YX + oxz * m.ZX;
		this.YY = oyx * m.XY + this.YY * m.YY + oyz * m.ZY;
		this.ZZ = ozx * m.XZ + ozy * m.YZ + this.ZZ * m.ZZ;
	}

	premultiply(m: Mat3) {
		const oxy = this.XY;
		const oyx = this.YX;
		const oyz = this.YZ;
		const ozy = this.ZY;
		const ozx = this.ZX;
		const oxz = this.XZ;

		this.XY = m.XX * oxy + m.XY * this.YY + m.XZ * ozy;
		this.XZ = m.XX * oxz + m.XY * oyz + m.XZ * this.ZZ;
		this.YX = m.YX * this.XX + m.YY * oyx + m.YZ * ozx;
		this.YZ = m.YX * oxz + m.YY * oyz + m.YZ * this.ZZ;
		this.ZX = m.ZX * this.XX + m.ZY * oyx + m.ZZ * ozx;
		this.ZY = m.ZX * oxy + m.ZY * this.YY + m.ZZ * ozy;

		this.XX = m.XX * this.XX + m.XY * oyx + m.XZ * ozx;
		this.YY = m.YX * oxy + m.YY * this.YY + m.YZ * ozy;
		this.ZZ = m.ZX * oxz + m.ZY * oyz + m.ZZ * this.ZZ;
	}

	translate(x: number, y: number, post: boolean) {
		let tm = new Mat3();
		tm.v02 = x;
		tm.v12 = y;

		if (post)
			this.postmultiply(tm);
		else
			this.premultiply(tm);
	}

	scale(x: number, y: number, post: boolean) {
		let tm = new Mat3();
		tm.v00 = x;
		tm.v11 = y;

		if (post)
			this.postmultiply(tm);
		else
			this.premultiply(tm);
	}

	rotate(angle: number, post: boolean) {
		let tm = new Mat3();
		const cosa = Math.cos(angle);
		const sina = Math.sin(angle);
		tm.v00 = cosa;
		tm.v01 = -sina;
		tm.v10 = sina;
		tm.v11 = cosa;

		if (post)
			this.postmultiply(tm);
		else
			this.premultiply(tm);
	}

	get avgScale(): number {
		let x = Math.sqrt(this.XX * this.XX + this.XY * this.XY);
		let y = Math.sqrt(this.YX * this.YX + this.YY * this.YY);
		return (x + y) / 2;
	}

	static newRotation2D(angleRadians: number): Mat3 {
		let m = new Mat3();
		m.v00 = Math.cos(angleRadians);
		m.v01 = -Math.sin(angleRadians);
		m.v10 = -m.v01;
		m.v11 = m.v00;
		return m;
	}

	toString(precision = 3): string {
		return "[[" + this.XX.toFixed(precision) + "," + this.XY.toFixed(precision) + "," + this.XZ.toFixed(precision) + "], " +
			"[" + this.YX.toFixed(precision) + "," + this.YY.toFixed(precision) + "," + this.YZ.toFixed(precision) + "], " +
			"[" + this.ZX.toFixed(precision) + "," + this.ZY.toFixed(precision) + "," + this.ZZ.toFixed(precision) + "]]";
	}
}