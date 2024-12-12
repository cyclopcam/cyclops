export class PinchZoom {
	// 1st finger position in canvas coordinates, on initial touch
	canvasX1 = 0;
	canvasY1 = 0;
	canvasX1_Latest = 0;
	canvasY1_Latest = 0;
	worldX1 = 0;
	worldY1 = 0;

	// 2nd finger position in canvas coordinates, on initial touch
	canvasX2 = 0;
	canvasY2 = 0;
	canvasX2_Latest = 0;
	canvasY2_Latest = 0;
	worldX2 = 0;
	worldY2 = 0;

	active = false;

	finger1Down(canvasX: number, canvasY: number, worldX: number, worldY: number) {
		this.canvasX1 = canvasX;
		this.canvasY1 = canvasY;
		this.canvasX1_Latest = canvasX;
		this.canvasY1_Latest = canvasY;
		this.worldX1 = worldX;
		this.worldY1 = worldY;
	}

	finger2Down(canvasX: number, canvasY: number, worldX: number, worldY: number) {
		this.canvasX2 = canvasX;
		this.canvasY2 = canvasY;
		this.canvasX2_Latest = canvasX;
		this.canvasY2_Latest = canvasY;
		this.worldX2 = worldX;
		this.worldY2 = worldY;
		this.active = true;
	}

	finger1Move(canvasX: number, canvasY: number) {
		this.canvasX1_Latest = canvasX;
		this.canvasY1_Latest = canvasY;
	}

	finger2Move(canvasX: number, canvasY: number) {
		this.canvasX2_Latest = canvasX;
		this.canvasY2_Latest = canvasY;
	}

	//compute(canvasX1: number, canvasY1: number, canvasX2: number, canvasY2: number): { scale: number, tx: number, ty: number } {
	compute(): { scale: number, tx: number, ty: number } {
		let lenOrg = Math.hypot(this.canvasX2 - this.canvasX1, this.canvasY2 - this.canvasY1);
		let lenNew = Math.hypot(this.canvasX2_Latest - this.canvasX1_Latest, this.canvasY2_Latest - this.canvasY1_Latest);

		let cx1 = (this.canvasX1 + this.canvasX2) / 2;
		let cy1 = (this.canvasY1 + this.canvasY2) / 2;
		let cx2 = (this.canvasX1_Latest + this.canvasX2_Latest) / 2;
		let cy2 = (this.canvasY1_Latest + this.canvasY2_Latest) / 2;

		let scale = lenNew / lenOrg;
		let tx = cx2 - cx1;
		let ty = cy2 - cy1;

		return { scale, tx, ty };
	}
}