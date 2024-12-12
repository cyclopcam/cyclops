import { Mat3 } from './mat';
import { clamp } from '../util/util';
import { Rect } from '../util/rect';
import { Vec2 } from './vec';

// XForm takes care of world space/view space transforms
// "World" space is the thing you're showing, such as an image, or a grid
// "Canvas" space is the pixels of the canvas
// Much of our internal state is private, to prevent an inconsistent state
// where inverse has not been computed. This was a genuine problem, not theoretical.
export class XForm {
	private _m = new Mat3(); // World to canvas
	private _inverse = new Mat3(); // Canvas to world (use makeInverse() to compute this)
	minScale = 1;
	maxScale = 20000;

	// If you have already constructed the forward and reverse matrices, then you can input them directly.
	// This avoids the costly Mat3 inversion.
	static dual(forward: Mat3, reverse: Mat3): XForm {
		let x = new XForm();
		x._m = forward;
		x._inverse = reverse;
		return x;
	}

	// WARNING. Do not mutate m, because then inverse will be out of sync.
	get m(): Mat3 {
		return this._m;
	}

	set m(v: Mat3) {
		this._m = v.clone();
		this.makeInverse();
	}

	get inverse(): Mat3 {
		return this._inverse;
	}

	private get scale(): number {
		return this.m.avgScale;
	}

	private set scale(v: number) {
		this.m.XX = v;
		this.m.XY = 0;
		this.m.YX = 0;
		this.m.YY = v;
	}

	private get tx(): number {
		return this.m.XZ;
	}

	private set tx(v: number) {
		this.m.XZ = v;
	}

	private get ty(): number {
		return this.m.YZ;
	}

	private set ty(v: number) {
		this.m.YZ = v;
	}

	getScale(): number {
		return this.scale;
	}

	getTx(): number {
		return this.tx;
	}

	getTy(): number {
		return this.ty;
	}

	makeInverse() {
		this._inverse = this.m.inverted();
	}

	worldToCanvasPt(worldPt: Vec2): Vec2 {
		return this.worldToCanvas(worldPt.x, worldPt.y);
	}

	worldToCanvas(x: number, y: number): Vec2 {
		return this.m.mulXY(x, y);
	}

	worldToCanvasLength(len: number): number {
		return len * this.scale;
	}

	worldToCanvasRect(world: Rect): Rect {
		let r = Rect.inverted();
		r.expandToFitPt(this.worldToCanvas(world.x1, world.y1));
		r.expandToFitPt(this.worldToCanvas(world.x2, world.y2));
		return r;
	}

	canvasToWorldPt(canvasPt: Vec2): Vec2 {
		return this.canvasToWorld(canvasPt.x, canvasPt.y);
	}

	canvasToWorld(x: number, y: number): Vec2 {
		return this.inverse.mulXY(x, y);
		//return new Vec2((x - this.tx) / this.scale, (y - this.ty) / this.scale);
	}

	canvasToWorldLength(len: number): number {
		return len / this.scale;
	}

	zoomRect(world: Rect, canvas: Rect, fill = 0.9) {
		let scale = Math.min(canvas.width / world.width, canvas.height / world.height) * fill;
		scale = clamp(scale, this.minScale, this.maxScale);
		this.scale = scale;
		this.tx = (canvas.x1 + canvas.x2) / 2 - scale * (world.x1 + world.x2) / 2;
		this.ty = (canvas.y1 + canvas.y2) / 2 - scale * (world.y1 + world.y2) / 2;
		this.makeInverse();
	}

	// zoom by a 'zoom' factor which is less than or greater than 1
	zoomPoint(x: number, y: number, zoom: number, snapScale = true) {
		let newScale = this.scale * zoom;
		if (snapScale)
			newScale = this.snapToPowerOf(newScale, 1.1);
		newScale = clamp(newScale, this.minScale, this.maxScale);
		this.zoomAroundPoint(x, y, newScale);
	}

	// zoom around a point, to a precise new scale
	zoomAroundPoint(x: number, y: number, newScale: number) {
		// constraint: canvasToImg(x,y) === canvasToImgM(x,y)
		// canvasToImg  is our original transformation from canvas coords to image coords
		// canvasToImgM is our modified transformation from canvas coords to image coords
		// in other words, the cursor must remain in the same place in image coords.

		const s = this.scale;
		const tx = this.tx;
		const ty = this.ty;
		const sM = newScale;

		//console.log('zoom to ', x, y, sM);

		// and all that remains, is to solve for txM and tyM
		// imgToCanvas(x) = x * s + tx
		// canvasToImg(x) = (x - tx) / s
		// from our constraint:
		// (x - txM) / sM = (x - tx) / s
		// x - txM = sM * (x - tx) / s
		// -txM = sM * (x - tx) / s - x
		// txM = x - sM * (x - tx) / s
		let txM = x - sM * (x - tx) / s;
		let tyM = y - sM * (y - ty) / s;
		txM = Math.round(txM);
		tyM = Math.round(tyM);

		this.scale = sM;
		this.tx = txM;
		this.ty = tyM;
		this.makeInverse();
	}

	// round scale to nearest power of 2
	snapToPowerOf2(x: number): number {
		x = Math.log2(x);
		x = Math.round(x);
		return Math.pow(2, x);
	}

	// round scale to nearest power of a number roundTo
	snapToPowerOf(x: number, roundTo: number): number {
		x = Math.log(x) / Math.log(roundTo);
		x = Math.round(x);
		return Math.pow(roundTo, x);
	}


}