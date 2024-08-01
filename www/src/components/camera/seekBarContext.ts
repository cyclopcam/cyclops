
import { spanIntersection } from "@/util/geom";
import { EventTile } from "./eventTile";
import { CachedEventTile, globalTileCache } from "./eventTileCache";
import { clamp } from "@/util/util";

import { BitsPerTile, BaseSecondsPerTile, MaxTileLevel } from "./eventTile";

// HistoryBar draws the lines at the bottom of a video which show the moments
// of interest when particular things were detected. For example, the bar might
// be white everywhere, but red where a person was detected.
// NOTE! This object gets made reactive, so don't store lots of state in here.
export class SeekBarContext {
	cameraID = 0;
	endTimeMS = new Date().getTime(); // Unix milliseconds at the end of the seek bar
	endTimeIsNow = false; // If our last seek call was seekToNow()
	zoomLevel = 3; // 2^zoom seconds per pixel. This can be an arbitrary real number.
	needsRender = false;

	constructor(cameraID = 0) {
		this.cameraID = cameraID;
	}

	seekToNow() {
		this.endTimeMS = new Date().getTime();
		this.endTimeIsNow = true;
	}

	seekTo(t: Date) {
		this.endTimeMS = t.getTime();
		this.endTimeIsNow = false;
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
		//console.log(`canvas size = ${canvas.width}x${canvas.height}`);
		let cx = canvas.getContext("2d");
		if (!cx) {
			return;
		}
		//cx.fillStyle = "rgba(255, 255, 255, 1)";
		//cx.fillRect(0, 0, canvas.width, canvas.height);
		// Work from right to left. Our right edge is 'endTime'.
		// Our time units are milliseconds.
		// Our spatial units are native device pixels.
		let secondsPerPixel = Math.pow(2, this.zoomLevel);
		let pixelsPerSecond = 1 / secondsPerPixel;
		let canvasWidth = canvas.width;
		let startTimeMS = this.pixelToTimeMS(0, canvasWidth, secondsPerPixel);

		//if (this.zoomLevel >= 10) console.log(`StartTime = ${new Date(startTimeMS).toISOString()}, EndTime = ${new Date(this.endTimeMS).toISOString()}`);

		// Try a few different tile levels to see which one gives us tiles *right now*, so that we
		// always get something reasonable on the screen, even when zooming in our out. But, on 
		// the pass where we're trying to render our ideal zoom level, make sure we fetch tiles that
		// are missing, so that subsequent re-renders will have the tiles they need.
		// Note that trying levels much higher than our current level is not a big penalty, because
		// the tiles get larger and larger, and thus the number of tiles that we need to investigate/fetch
		// become smaller and smaller (until it's usually just 1 or 2 tiles).
		let idealLevel = Math.floor(this.zoomLevel);
		idealLevel = clamp(idealLevel, 0, MaxTileLevel);
		let bestCoverage = 0;
		let bestLevel = idealLevel;
		for (let tileLevel = idealLevel - 2; tileLevel < idealLevel + 3; tileLevel++) {
			let coverage = this.cachedTileCoverage(startTimeMS, this.endTimeMS, canvasWidth, pixelsPerSecond, tileLevel);
			if (coverage > bestCoverage) {
				bestCoverage = coverage;
				bestLevel = tileLevel;
			} else if (coverage === bestCoverage) {
				// If coverage is equal, choose the level closest to the ideal level
				if (Math.abs(tileLevel - idealLevel) < Math.abs(bestLevel - idealLevel)) {
					bestLevel = tileLevel;
				}
			}
		}

		//console.log(`zoomLevel = ${this.zoomLevel}, idealLevel = ${idealLevel}, bestLevel = ${bestLevel}, bestCoverage = ${bestCoverage}`);

		// Make sure we have tile fetches queued for the ideal level
		{
			let { startTileIdx, endTileIdx } = SeekBarContext.tileSpan(startTimeMS, this.endTimeMS, idealLevel);
			for (let tileIdx = startTileIdx; tileIdx < endTileIdx; tileIdx++) {
				globalTileCache.getTile(this.cameraID, idealLevel, tileIdx, onTileFetched);
			}
		}

		// Render tiles using the best level. Here we don't queue up tile fetches.
		let { startTileIdx, endTileIdx } = SeekBarContext.tileSpan(startTimeMS, this.endTimeMS, bestLevel);
		for (let tileIdx = startTileIdx; tileIdx < endTileIdx; tileIdx++) {
			let tile = globalTileCache.getTile(this.cameraID, bestLevel, tileIdx);
			if (tile) {
				this.renderTile(cx, tile, canvasWidth, pixelsPerSecond);
			}
		}
	}

	static tileSpan(startTimeMS: number, endTimeMS: number, tileLevel: number): { startTileIdx: number, endTileIdx: number } {
		// The extra 1000 in the following lines is to go from milliseconds to seconds.

		// NOTE! Bit shifts in Javascript operate at 32 bits, so the following formulation does NOT work,
		// because you'll overflow into negative numbers once the shift gets to about 12 or 13.
		// let startTileIdx = Math.floor(startTimeMS / (1000 * BaseSecondsPerTile << tileLevel));

		let startTileIdx = Math.floor((startTimeMS / 1000) / (BaseSecondsPerTile << tileLevel)); // inclusive.
		let endTileIdx = Math.ceil((endTimeMS / 1000) / (BaseSecondsPerTile << tileLevel)); // exclusive
		// Clamp the number of renderings/fetches, just in case we screw something up.
		// 10 should be PLENTY of a high enough limit when tileLevel is close to zoomLevel.
		// But since we sometimes render tiles from a lower level (eg when user is busy zooming out),
		// we relax this limit a bit.
		startTileIdx = Math.max(startTileIdx, endTileIdx - 40);
		return { startTileIdx, endTileIdx };
	}

	// Returns the percentage (0..1) of the screen that is covered by cached tiles at the given time span and level.
	cachedTileCoverage(startTimeMS: number, endTimeMS: number, canvasWidth: number, pixelsPerSecond: number, tileLevel: number): number {
		let { startTileIdx, endTileIdx } = SeekBarContext.tileSpan(startTimeMS, endTimeMS, tileLevel);
		let nPixels = 0;
		for (let tileIdx = startTileIdx; tileIdx < endTileIdx; tileIdx++) {
			let tile = globalTileCache.getTile(this.cameraID, tileLevel, tileIdx);
			if (tile) {
				let { x1, x2 } = this.renderedTileBounds(tile, canvasWidth, pixelsPerSecond);
				let [s1, s2] = spanIntersection(x1, x2, 0, canvasWidth);
				nPixels += s2 - s1;
			}
		}
		return nPixels / canvasWidth;
	}

	renderedTileBounds(tile: EventTile, canvasWidth: number, pixelsPerSecond: number): { x1: number, x2: number } {
		let x1 = this.timeMSToPixel(tile.startTimeMS, canvasWidth, pixelsPerSecond);
		let x2 = this.timeMSToPixel(tile.endTimeMS, canvasWidth, pixelsPerSecond);
		return { x1, x2 }
	}

	renderTile(cx: CanvasRenderingContext2D, tile: EventTile, canvasWidth: number, pixelsPerSecond: number) {
		let dpr = window.devicePixelRatio;
		let { x1: tx1, x2: tx2 } = this.renderedTileBounds(tile, canvasWidth, pixelsPerSecond);
		let bitWidth = (tx2 - tx1) / BitsPerTile;
		let classes = ["person", "car", "truck"];
		let colors = ["rgba(255, 40, 0, 1)", "rgba(0, 255, 0, 1)", "rgba(150, 100, 255, 1)"];
		let y = 4.5;
		let lineHeight = 3 * dpr;
		let bitWindowCount = new Uint8Array(BitsPerTile);
		for (let icls = 0; icls < classes.length; icls++) {
			cx.fillStyle = colors[icls];
			let bitmap = tile.classes[classes[icls]];
			if (bitmap) {
				SeekBarContext.countBitsInSlidingWindow(bitmap, bitWindowCount, 5);
				//console.log("window", bitWindowCount);
				let state = 0;
				let x1 = tx1;
				let x2 = tx1;
				for (let bit = 0; bit <= BitsPerTile; bit++) {
					if (bit === BitsPerTile || EventTile.getBit(bitmap, bit) !== state) {
						if (state === 1) {
							let density = bit === BitsPerTile ? bitWindowCount[BitsPerTile - 1] : bitWindowCount[bit];
							let rx1 = x1;
							let width = x2 - x1;
							if (density < 3) {
								// Boost width of detections that are sparse, so that the human eye doesn't miss them.
								rx1 -= 2;
								width += 3;
							}
							cx.fillRect(rx1, y, width, lineHeight);
						}
						state = state ? 0 : 1;
						x1 = x2;
					}
					x2 += bitWidth;
				}
			}
			y += lineHeight + 1;
		}
	}

	// Count the number of bits in a sliding window of windowSize size, and write that number
	// into bitWindowCount.
	static countBitsInSlidingWindow(bitmap: Uint8Array, bitWindowCount: Uint8Array, windowSize: number) {
		let n = bitmap.length * 8;
		let count = 0;
		for (let i = 0; i < n; i++) {
			if (i >= windowSize && EventTile.getBit(bitmap, i - windowSize)) {
				count--;
			}
			if (EventTile.getBit(bitmap, i)) {
				count++;
			}
			bitWindowCount[i] = count;
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
}