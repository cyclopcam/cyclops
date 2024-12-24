import { Vec2 } from "./vec";

export class PinchZoomState {
	constructor(public scale: number, public tx: number, public ty: number) { }

	equals(other: PinchZoomState): boolean {
		return this.scale === other.scale && this.tx === other.tx && this.ty === other.ty;
	}
}

export class PinchZoom {
	// 1st finger position in canvas coordinates
	private pointer1Down = new Vec2(0, 0);
	private pointer1Latest = new Vec2(0, 0);

	// 2nd finger position in canvas coordinates
	private pointer2Down = new Vec2(0, 0);
	private pointer2Latest = new Vec2(0, 0);

	private scale_down = 1;
	private tx_down = 0;
	private ty_down = 0;

	// Map from pointerId to either 0 or 1 (which is the 1st or 2nd finger)
	private pointerMap = new Map<number, number>();

	scale = 1;
	tx = 0;
	ty = 0;

	active = false;

	reset() {
		this.scale = 1;
		this.tx = 0;
		this.ty = 0;
		this.pointerMap.clear();
		this.active = false;
	}

	isIdentity(): boolean {
		return this.scale === 1 && this.tx === 0 && this.ty === 0;
	}

	get state(): PinchZoomState {
		return new PinchZoomState(this.scale, this.tx, this.ty);
	}

	set state(state: PinchZoomState) {
		this.scale = state.scale;
		this.tx = state.tx;
		this.ty = state.ty;
	}

	onPointerDown(pointerId: number, canvasX: number, canvasY: number) {
		let start = false;
		this.pointerMap.set(pointerId, this.pointerMap.size);
		if (this.pointerMap.size === 1) {
			this.pointer1Down.set(canvasX, canvasY);
			this.pointer1Latest.set(canvasX, canvasY);
			if (!this.isIdentity()) {
				// could be panning
				start = true;
			}
		} else if (this.pointerMap.size === 2) {
			//console.log("2nd finger down");
			this.pointer2Down.set(canvasX, canvasY);
			this.pointer2Latest.set(canvasX, canvasY);
			start = true;
		}
		if (start) {
			this.active = true;
			this.scale_down = this.scale;
			this.tx_down = this.tx;
			this.ty_down = this.ty;
		}
	}

	onPointerMove(pointerId: number, canvasX: number, canvasY: number) {
		let p = this.pointerMap.get(pointerId);
		if (p === 0) {
			this.pointer1Latest.set(canvasX, canvasY);
		} else if (p === 1) {
			this.pointer2Latest.set(canvasX, canvasY);
		}
		if (this.active) {
			this.computePinchZoomOrPan();
		}
	}

	onPointerUp(pointerId: number) {
		this.pointerMap.delete(pointerId);
		this.active = false;
	}

	onWheel(deltaY: number, canvasX: number, canvasY: number) {
		this.zoomAroundPoint(canvasX, canvasY, this.scale * (1 - deltaY / 1000));
	}

	computePinchZoomOrPan() {
		if (this.pointerMap.size === 2) {
			// pinch-zoom
			let lenOrg = this.pointer1Down.distance(this.pointer2Down);
			let lenNew = this.pointer1Latest.distance(this.pointer2Latest);

			// The world coordinates must remain constant during a pinch zoom, so we need to solve for tx/ty.
			// X and Y are the same, so we just work it out for the X dimension.

			// The fundamental transform function is:
			// canvas = world * scale + tx

			// So we can derive it what we need:
			// world = (canvas - tx) / scale
			// world_1_org = (canvas_1_org - tx_org) / scale_org
			// world_2_org = (canvas_2_org - tx_org) / scale_org
			// world_1_new = (canvas_1_new - tx_new) / scale_new
			// world_2_new = (canvas_2_new - tx_new) / scale_new

			// We add world_1_new and world_2_new to get a function that combines both points,
			// which we can simplify down to get tx_new:
			// world_1_new + world_2_new = (canvas_1_new - tx_new) / scale_new + (canvas_2_new - tx_new) / scale_new
			// world_1_new + world_2_new = (canvas_1_new - tx_new + canvas_2_new - tx_new) / scale_new
			// scale_new * (world_1_new + world_2_new) = canvas_1_new + canvas_2_new - 2 * tx_new
			// tx_new = (canvas_1_new + canvas_2_new - scale_new * (world_1_new + world_2_new)) / 2

			// But remember the world points do not change, so we can substitute world org for world new
			// tx_new = (canvas_1_new + canvas_2_new - scale_new * (world_1_org + world_2_org)) / 2

			let scale_new = this.scale_down * lenNew / lenOrg;
			let world_1_x = (this.pointer1Down.x - this.tx_down) / this.scale_down;
			let world_1_y = (this.pointer1Down.y - this.ty_down) / this.scale_down;
			let world_2_x = (this.pointer2Down.x - this.tx_down) / this.scale_down;
			let world_2_y = (this.pointer2Down.y - this.ty_down) / this.scale_down;
			let tx_new = (this.pointer1Latest.x + this.pointer2Latest.x - scale_new * (world_1_x + world_2_x)) / 2;
			let ty_new = (this.pointer1Latest.y + this.pointer2Latest.y - scale_new * (world_1_y + world_2_y)) / 2;

			this.scale = scale_new;
			this.tx = tx_new;
			this.ty = ty_new;
		} else if (this.pointerMap.size === 1) {
			// pan
			let dx = this.pointer1Latest.x - this.pointer1Down.x;
			let dy = this.pointer1Latest.y - this.pointer1Down.y;
			this.tx = this.tx_down + dx;
			this.ty = this.ty_down + dy;
		}
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
	}

}