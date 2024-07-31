
import { EventTile } from "./eventTile";
import { CachedEventTile, globalTileCache } from "./eventTileCache";

const BaseSecondsPerTile = 1024;

// HistoryBar draws the lines at the bottom of a video which show the moments
// of interest when particular things were detected. For example, the bar might
// be white everywhere, but red where a person was detected.
// NOTE! This object gets made reactive, so don't store lots of state in here.
export class SeekBarContext {
	cameraID = 0;
	endTimeMS = new Date().getTime(); // Unix milliseconds at the end of the seek bar
	tileLevel = 7; // 2^tileLevel seconds per bit. At level 0, 1 bit per second. This must be an integer.
	zoomLevel = 4; // 2^zoom seconds per pixel. This can be an arbitrary real number.
	needsRender = false;

	constructor(cameraID = 0) {
		this.cameraID = cameraID;
	}

	seekToNow() {
		this.endTimeMS = new Date().getTime();
		//console.log("seekToNow, cameraID = ", this.cameraID);
	}

	render(canvas: HTMLCanvasElement) {
		let reRender = () => {
			if (this.needsRender) {
				this.render(canvas);
			}
		}
		// This gets called when a tile is loaded
		let onTileFetched = (tile: CachedEventTile) => {
			this.needsRender = true;
			requestAnimationFrame(reRender);
		}

		this.needsRender = false;
		let dpr = window.devicePixelRatio;
		canvas.width = canvas.clientWidth * dpr;
		canvas.height = canvas.clientHeight * dpr;
		//console.log("canvas width: ", canvas.width);
		let cx = canvas.getContext("2d")!;
		cx.fillStyle = "rgba(255, 255, 255, 1)";
		cx.fillRect(0, 0, canvas.width, canvas.height);
		// Work from right to left. Our right edge is 'endTime'.
		// Our time units are milliseconds.
		// Our spatial units are native device pixels.
		let secondsPerPixel = Math.pow(2, this.zoomLevel);
		let pixelsPerSecond = 1 / secondsPerPixel;
		let canvasWidth = canvas.width;
		let startTimeMS = this.pixelToTimeMS(0, canvasWidth, secondsPerPixel);
		// extra 1000 in the following lines is to go from milliseconds to seconds.
		let startTileIdx = Math.floor((startTimeMS) / (1000 * BaseSecondsPerTile << this.tileLevel)); // inclusive.
		let endTileIdx = Math.ceil((this.endTimeMS) / (1000 * BaseSecondsPerTile << this.tileLevel)); // exclusive
		// Clamp the number of renderings/fetches, just in case we screw something up.
		// 10 should be PLENTY of a high enough limit.
		startTileIdx = Math.max(startTileIdx, endTileIdx - 10);
		for (let tileIdx = startTileIdx; tileIdx < endTileIdx; tileIdx++) {
			//console.log("getTile ", this.cameraID);
			let tile = globalTileCache.getTile(this.cameraID, this.tileLevel, tileIdx, onTileFetched);
			if (tile) {
				this.renderTile(cx, tile, canvasWidth, pixelsPerSecond);
			}
		}
	}

	renderTile(cx: CanvasRenderingContext2D, tile: EventTile, canvasWidth: number, pixelsPerSecond: number) {
		let tx1 = this.timeMSToPixel(tile.startTimeMS, canvasWidth, pixelsPerSecond);
		let tx2 = this.timeMSToPixel(tile.endTimeMS, canvasWidth, pixelsPerSecond);
		//console.log("Tile width in pixels = ", tx2 - tx1);
		let bitWidth = (tx2 - tx1) / 1024;
		let classes = ["person", "car", "truck"];
		let colors = ["rgba(205, 0, 0, 1)", "rgba(0, 105, 0, 1)", "rgba(0, 0, 155, 1)"];
		let y = 0;
		let lineHeight = 8;
		for (let icls = 0; icls < classes.length; icls++) {
			cx.fillStyle = colors[icls];
			let bitmap = tile.classes[classes[icls]];
			if (bitmap) {
				let state = 0;
				let x1 = tx1;
				let x2 = tx1;
				for (let bit = 0; bit <= 1024; bit++) {
					if (bit === 1024 || EventTile.getBit(bitmap, bit) !== state) {
						if (state === 1) {
							//console.log(x1, x2);
							cx.fillRect(x1, y, x2 - x1, lineHeight);
						}
						state = state ? 0 : 1;
						x1 = x2;
					}
					x2 += bitWidth;
				}
			}
			y += lineHeight;
		}
	}

	timeMSToPixel(timeMS: number, canvasWidth: number, pixelsPerSecond?: number): number {
		if (pixelsPerSecond === undefined) {
			pixelsPerSecond = 1 / Math.pow(2, this.zoomLevel);
		}
		return ((timeMS - this.endTimeMS) / 1000) * pixelsPerSecond + canvasWidth;
	}

	// px: Distance in pixels from the left edge (left edge = 0)
	// canvasWidth: Width of canvas in pixels
	// secondsPerPixel: If omitted, calculated from zoomLevel
	// Returns time in unix milliseconds.
	pixelToTimeMS(px: number, canvasWidth: number, secondsPerPixel?: number): number {
		if (secondsPerPixel === undefined) {
			secondsPerPixel = Math.pow(2, this.zoomLevel);
		}
		return this.endTimeMS + (px - canvasWidth) * secondsPerPixel * 1000;
	}

	static async downloadTiles(cameraID: number, canvasWidthPx: number, startTime: Date, endTime: Date): Promise<EventTile[]> {
		console.log(`Download from ${startTime} to ${endTime}`);
		let startUnixSecond = startTime.getTime() / 1000;
		let endUnixSecond = endTime.getTime() / 1000;
		let numSeconds = endUnixSecond - startUnixSecond;
		numSeconds = Math.max(numSeconds, 1);
		if (canvasWidthPx <= 1) {
			return [];
		}
		// The tile API wants a level and start and end tile indices, so we need to do that conversion here.
		// Let's start.
		// At level zero, tiles are 1 second per pixel.
		// We figure out the right level, and then we fetch tiles that span our desired time range.
		let pixelsPerSecond = canvasWidthPx / numSeconds;
		let level = Math.floor(Math.log2(pixelsPerSecond));
		level = Math.max(level, 0);
		let startIdx = Math.floor(startUnixSecond / (BaseSecondsPerTile << level));
		let endIdx = Math.ceil(endUnixSecond / (BaseSecondsPerTile << level));
		return await EventTile.fetchTiles(cameraID, level, startIdx, endIdx);
	}
}