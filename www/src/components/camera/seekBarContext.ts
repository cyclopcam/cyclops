
import { spanIntersection } from "@/util/geom";
import { EventTile } from "./eventTile";
import { CachedEventTile, globalTileCache } from "./eventTileCache";
import { clamp, dateTime } from "@/util/util";

import { BitsPerTile, BaseSecondsPerTile, MaxTileLevel } from "./eventTile";

// SeekBar draws the lines at the bottom of a video which show the moments
// of interest when particular things were detected. For example, the bar might
// be white everywhere, but red where a person was detected.
// The seek bar represents two different things related to time:
// 1. The start/end time of the bar
// 2. The current video playback position
// We call #1 the "pan"
// We cann #2 the "seek"
// NOTE! This object gets made reactive, so don't store lots of state in here.
export class SeekBarContext {
	cameraID = 0;
	panTimeEndMS = new Date().getTime(); // Unix milliseconds at the end of the seek bar
	panTimeEndIsNow = false; // If our last seek call was seekToNow()
	zoomLevel = 3; // 2^zoom seconds per pixel. This can be an arbitrary real number.
	desiredSeekPosMS = new Date().getTime(); // Unix milliseconds of the desired video seek position
	actualSeekPosMS = new Date().getTime(); // Unix milliseconds of the actual playback position
	needsRender = false;

	constructor(cameraID = 0) {
		this.cameraID = cameraID;
	}

	// Set the end time to now
	panToNow() {
		this.panTimeEndMS = new Date().getTime();
		this.panTimeEndIsNow = true;
	}

	// Set the end time to 't'
	panTo(t: Date) {
		this.panToMillisecond(t.getTime());
	}

	// Set the end time to 'ms' (unix milliseconds)
	panToMillisecond(ms: number) {
		this.panTimeEndMS = ms;
		this.panTimeEndIsNow = false;
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
		let canvasHeight = canvas.height;
		let startTimeMS = this.pixelToTimeMS(0, canvasWidth, secondsPerPixel);

		//if (this.zoomLevel >= 10) console.log(`StartTime = ${new Date(startTimeMS).toISOString()}, EndTime = ${new Date(this.endTimeMS).toISOString()}`);

		this.renderTimeMarkers(cx, canvasWidth, canvasHeight, startTimeMS, pixelsPerSecond);

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
			let coverage = this.cachedTileCoverage(startTimeMS, this.panTimeEndMS, canvasWidth, pixelsPerSecond, tileLevel);
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
			let { startTileIdx, endTileIdx } = SeekBarContext.tileSpan(startTimeMS, this.panTimeEndMS, idealLevel);
			for (let tileIdx = startTileIdx; tileIdx < endTileIdx; tileIdx++) {
				globalTileCache.getTile(this.cameraID, idealLevel, tileIdx, onTileFetched);
			}
		}

		// Render tiles using the best level. Here we don't queue up tile fetches.
		let { startTileIdx, endTileIdx } = SeekBarContext.tileSpan(startTimeMS, this.panTimeEndMS, bestLevel);
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

	renderTimeMarkers(cx: CanvasRenderingContext2D, canvasWidth: number, canvasHeight: number, startTimeMS: number, pixelsPerSecond: number) {
		let startS = startTimeMS / 1000;
		let endS = this.panTimeEndMS / 1000;

		//let secondsPerPixel = ((this.endTimeMS - startTimeMS) / 1000) / canvasWidth;
		let totalH = (endS - startS) / (60 * 60);
		//console.log(totalHours);

		// Interval between markers, in seconds
		let colors = ["rgba(255, 255, 255, 0.3)", "rgba(255, 255, 255, 0.5)"];
		let intervalS = 1;
		if (totalH * 60 < 15) {
			// minute
			intervalS = 60;
		} else if (totalH < 0.5) {
			// 5 minutes
			intervalS = 5 * 60;
		} else if (totalH < 2) {
			// 15 minutes
			intervalS = 15 * 60;
		} else if (totalH < 24) {
			// hours
			intervalS = 60 * 60;
		} else if (totalH < 24 * 7 * 60) {
			// days
			intervalS = 60 * 60 * 24;
			colors = ["rgba(255, 255, 190, 0.3)", "rgba(255, 255, 190, 0.5)"];
		} else {
			// Maybe do week and/or month markers?
			return;
		}

		let dpr = window.devicePixelRatio;
		let height = dpr * 4;

		let istart = Math.floor(startS / intervalS);
		let iend = Math.ceil(endS / intervalS);
		let showText = iend - istart <= 12;
		//console.log(istart, iend);
		for (let i = istart; i < iend; i++) {
			let t1 = this.timeMSToPixel(i * intervalS * 1000, canvasWidth, pixelsPerSecond);
			let t2 = this.timeMSToPixel((i + 1) * intervalS * 1000, canvasWidth, pixelsPerSecond);
			//console.log(t1, t2);
			cx.fillStyle = colors[i % 2];
			cx.fillRect(t1, canvasHeight - height, t2 - t1, canvasHeight);
			if (showText) {
				let d = new Date(i * intervalS * 1000);
				let h = d.getHours();
				let minutes: string | number = d.getMinutes();
				minutes = minutes === 0 ? "00" : minutes < 10 ? "0" + minutes : minutes;
				cx.font = `${9 * dpr}px -apple-system, system-ui, sans-serif`;
				cx.textAlign = "center";
				cx.fillStyle = "rgba(255, 255, 255, 0.5)";
				let text = '';
				if (intervalS <= 60 * 60) {
					text = h + ":" + minutes;
				} else if (intervalS === 60 * 60 * 24) {
					text = 'DoM';
				}
				cx.fillText(text, t1, canvasHeight - height - 5 * dpr);
			}
		}
	}

	renderTile(cx: CanvasRenderingContext2D, tile: EventTile, canvasWidth: number, pixelsPerSecond: number) {
		let dpr = window.devicePixelRatio;
		let { x1: tx1, x2: tx2 } = this.renderedTileBounds(tile, canvasWidth, pixelsPerSecond);
		let bitWidth = (tx2 - tx1) / BitsPerTile;
		let classes = ["person", "car", "truck"];
		let colors = ["rgba(255, 40, 0, 1)", "rgba(0, 255, 0, 1)", "rgba(150, 100, 255, 1)"];
		let y = 4.5;
		let lineHeight = 3 * dpr;
		//let boostThreshold = 2 * dpr;
		let boostThreshold = 4; // this is coupled to the sliding window size. If you make the window size bigger, you should make this bigger too.
		let boostLeft = 1.0 * dpr;
		let boostWidth = 1.75 * boostLeft; // should be about 2x boostLeft to keep the dot unbiased
		let bitWindowCount = new Uint8Array(BitsPerTile);
		for (let icls = 0; icls < classes.length; icls++) {
			cx.fillStyle = colors[icls];
			let bitmap = tile.classes[classes[icls]];
			if (bitmap) {
				SeekBarContext.countBitsInSlidingWindow(bitmap, bitWindowCount, 3);
				//console.log(this.cameraID, "window", bitWindowCount);
				let state = 0;
				let x1 = tx1;
				let x2 = tx1;
				for (let bit = 0; bit <= BitsPerTile; bit++) {
					if (bit === BitsPerTile || EventTile.getBit(bitmap, bit) !== state) {
						if (state === 1) {
							let density = bit === BitsPerTile ? bitWindowCount[BitsPerTile - 1] : bitWindowCount[bit];
							let rx1 = x1;
							let width = x2 - x1;
							if (density < boostThreshold) {
								// Boost width of detections that are sparse, so that the human eye doesn't miss them.
								rx1 -= boostLeft;
								width += boostWidth;
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
		return ((timeMS - this.panTimeEndMS) / 1000) * pixelsPerSecond + canvasWidth;
	}

	// px: Distance in pixels from the left edge (left edge = 0)
	// canvasWidth: Width of canvas in pixels
	// secondsPerPixel: If omitted, calculated from zoomLevel
	// Returns time in unix milliseconds.
	pixelToTimeMS(px: number, canvasWidth: number, secondsPerPixel?: number): number {
		if (secondsPerPixel === undefined) {
			secondsPerPixel = Math.pow(2, this.zoomLevel);
		}
		return this.panTimeEndMS + (px - canvasWidth) * secondsPerPixel * 1000;
	}
}

// Represents a mapping between two different 1-dimensional coordinate systems.
// In our case of the seek bar, the two different coordinate systems are:
// 1. Time (unix milliseconds)
// 2. Pixels (from left edge of canvas)
export class SeekBarTransform {
	pixelsPerSecond = 1;
	leftEdgeTimeMS = 0;
	canvasWidth = 0;

	static pixelsPerSecondToZoomLevel(pixelsPerSecond: number): number {
		return Math.log2(1 / pixelsPerSecond);
	}

	static fromZoomLevelAndRightEdge(zoomLevel: number, rightEdgeTimeMS: number, canvasWidth: number): SeekBarTransform {
		let transform = new SeekBarTransform();
		transform.pixelsPerSecond = 1 / Math.pow(2, zoomLevel);
		transform.leftEdgeTimeMS = rightEdgeTimeMS - (canvasWidth / transform.pixelsPerSecond) * 1000;
		transform.canvasWidth = canvasWidth;
		return transform;
	}

	// Given a time in milliseconds, return the distance in pixels from the left edge.
	timeToPixel(timeMS: number): number {
		return ((timeMS - this.leftEdgeTimeMS) / 1000) * this.pixelsPerSecond;
	}

	// Given pixels from the left edge, return the time in milliseconds.
	pixelToTime(px: number): number {
		return (px / this.pixelsPerSecond) * 1000 + this.leftEdgeTimeMS;
	}
}