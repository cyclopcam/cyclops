
import { spanIntersection } from "@/util/geom";
import { EventTile, tileEndTimeMS, tileStartTimeMS } from "./eventTile";
import { CachedEventTile, globalTileCache } from "./eventTileCache";
import { clamp, monthNamesShort, zeroPad } from "@/util/util";

import { BitsPerTile, BaseSecondsPerTile, MaxTileLevel } from "./eventTile";
import { SnapSeekState } from "./snapSeek";

// SeekBar draws the lines at the bottom of a video which show the moments
// of interest when particular things were detected. For example, the bar might
// be white everywhere, but red where a person was detected.
// The seek bar represents two different things related to time:
// 1. The start/end time of the bar
// 2. The current video playback position
// We call #1 the "pan"
// We call #2 the "seek"
// NOTE! This object gets made reactive, so don't store lots of state in here.
export class SeekBarContext {
	cameraID = 0;
	panTimeEndMS = new Date().getTime(); // Unix milliseconds at the end of the seek bar
	panTimeEndIsNow = false; // If our last seek call was seekToNow()
	zoomLevel = 3; // 2^zoom seconds per pixel. This can be an arbitrary real number. Higher zoom level = more zoomed out

	// Unix milliseconds of the desired video seek position, or 0 if no explicit seek (i.e. seek to now)
	// This is the place where the user's finger (or mouse cursor) is actually pointing, before we've taken snapping or auto-play into account.
	// See also actualSeekPosMS.
	desiredSeekPosMS = 0;

	needsRender = false;
	snap = new SnapSeekState();
	classes = ["person", "car", "truck"];
	colors = ["rgba(255, 40, 0, 1)", "rgba(0, 255, 0, 1)", "rgba(150, 100, 255, 1)"];

	constructor(cameraID = 0) {
		this.cameraID = cameraID;
		this.reset();
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

	seekToNow() {
		this.desiredSeekPosMS = 0;
		this.snap.posMS = 0;
	}

	seekToMillisecond(ms: number) {
		this.desiredSeekPosMS = ms;
	}

	reset() {
		this.panToNow();
		this.seekToNow();
		this.zoomLevel = 3;
	}

	setZoomLevel(zoomLevel: number) {
		zoomLevel = clamp(zoomLevel, -7, 15);
		this.zoomLevel = zoomLevel;
		//console.log("zoomLevel = " + zoomLevel);
	}

	allowSnap(): boolean {
		return this.zoomLevel > -1.5;
	}

	// The position where we want the video frame
	actualSeekPosMS(): number {
		if (this.snap.posMS !== 0) {
			return this.snap.posMS;
		}
		return this.desiredSeekPosMS;
	}

	// schedule a render on next animation frame
	invalidate(canvas: HTMLCanvasElement) {
		this.needsRender = true;
		requestAnimationFrame(() => this.render(canvas));
	}

	render(canvas: HTMLCanvasElement) {
		let reRender = () => {
			if (this.needsRender) {
				this.render(canvas);
			}
		}
		// This gets called when a tile is loaded (or the server says it has no tile for this time)
		let onTileFetched = (tile: CachedEventTile | undefined) => {
			this.needsRender = true;
			requestAnimationFrame(reRender);
		}

		this.needsRender = false;
		// If you change the canvas native size here, be sure to also change pxToCanvas() in SeekBar.vue,
		// because that code assumes clientWidth * DPR.
		let dpr = window.devicePixelRatio;
		canvas.width = canvas.clientWidth * dpr;
		canvas.height = canvas.clientHeight * dpr;
		//console.log(`canvas size = ${canvas.clientWidth * dpr} x ${canvas.clientHeight * dpr} -> ${canvas.width} x ${canvas.height}`);
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

		// Render the future in a different color
		let futureColor = "#555"
		cx.fillStyle = futureColor;
		let futureX = this.timeMSToPixel(new Date().getTime(), canvasWidth, pixelsPerSecond);
		cx.fillRect(futureX, 0, canvasWidth - futureX + 1, canvasHeight);

		//if (this.zoomLevel >= 10) console.log(`StartTime = ${new Date(startTimeMS).toISOString()}, EndTime = ${new Date(this.endTimeMS).toISOString()}`);

		this.renderTimeMarkers(cx, canvasWidth, canvasHeight, startTimeMS, pixelsPerSecond);

		// Try a few different tile levels to see which one gives us tiles *right now*, so that we
		// always get something reasonable on the screen, even when zooming in our out. But, on 
		// the pass where we're trying to render our ideal zoom level, make sure we fetch tiles that
		// are missing, so that subsequent re-renders will have the tiles they need.
		// Note that trying levels much higher than our current level is not a big penalty, because
		// the tiles get larger and larger, and thus the number of tiles that we need to investigate/fetch
		// become smaller and smaller (until it's usually just 1 or 2 tiles).
		// We try for tile levels very high up (eg +5), because it's an awful experience to zoom in
		// and then have your event disappear.
		let idealLevel = Math.floor(this.zoomLevel);
		idealLevel = clamp(idealLevel, 0, MaxTileLevel);
		let bestCoverage = 0;
		let bestLevel = idealLevel;
		for (let tileLevel = idealLevel - 2; tileLevel < idealLevel + 5; tileLevel++) {
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

		// Make sure we have tile fetches queued for the ideal level (not just the level that we're about to render, which could be different).
		{
			let { startTileIdx, endTileIdx } = SeekBarContext.tileSpan(startTimeMS, this.panTimeEndMS, idealLevel);
			for (let tileIdx = startTileIdx; tileIdx < endTileIdx; tileIdx++) {
				globalTileCache.getTile(this.cameraID, idealLevel, tileIdx, onTileFetched);
			}
		}

		// Get video start time, so that we don't render event bits which occurred so far in the past
		// that their video has already been erased.
		let videoStartTime = globalTileCache.cameraVideoStartTime[this.cameraID];

		// Render tiles using the best level. Here we don't queue up tile fetches.
		let { startTileIdx, endTileIdx } = SeekBarContext.tileSpan(startTimeMS, this.panTimeEndMS, bestLevel);
		for (let tileIdx = startTileIdx; tileIdx < endTileIdx; tileIdx++) {
			let tile = globalTileCache.getTile(this.cameraID, bestLevel, tileIdx);
			if (tile) {
				this.renderTile(cx, tile, canvasWidth, pixelsPerSecond, videoStartTime);
			} else if (globalTileCache.isFetching(this.cameraID, bestLevel, tileIdx)) {
				// Render a grey rectangle to show that data is busy loading.
				let x1 = this.timeMSToPixel(tileStartTimeMS(bestLevel, tileIdx), canvasWidth, pixelsPerSecond);
				let x2 = this.timeMSToPixel(tileEndTimeMS(bestLevel, tileIdx), canvasWidth, pixelsPerSecond);
				cx.fillStyle = futureColor;
				cx.fillRect(x1, 0, x2 - x1, 20);
			}
		}
		//console.log(`Render ${endTileIdx - startTileIdx} tiles at level ${bestLevel}`);

		let snapClassIdx = this.classes.indexOf(this.snap.detectedClass);
		let haveSnap = snapClassIdx !== -1 && this.snap.posMS !== 0;

		// Render seek bar
		if (this.desiredSeekPosMS !== 0) {
			let desiredSeekPx = this.timeMSToPixel(this.desiredSeekPosMS, canvasWidth, pixelsPerSecond);
			cx.fillStyle = "rgba(255, 255, 255, 1)";
			if (haveSnap) {
				cx.fillStyle = "rgba(255, 255, 255, 0.4)";
			}
			let w = 1;
			cx.fillRect(desiredSeekPx - w, 0, 2 * w, canvasHeight);
		}

		// Render snapped seek position
		if (haveSnap) {
			let posPx = this.timeMSToPixel(this.snap.posMS, canvasWidth, pixelsPerSecond);
			cx.fillStyle = "rgba(40, 80, 240, 1)";
			let w = 0.5 * dpr;
			cx.fillRect(posPx - w, 0, 2 * w, canvasHeight);
			let boxSize = 8;
			let yPad = 1 * dpr;
			let { lineHeight, y } = this.tileTimelineSpan(this.snap.detectedClass)!;
			cx.fillStyle = this.colors[snapClassIdx];
			cx.fillRect(posPx - boxSize / 2, y - yPad, boxSize, lineHeight + yPad * 2);
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
		// 40 should be PLENTY of a high enough limit when tileLevel is close to zoomLevel.
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

	// If overshoot, then include endTime
	computeMonthStartsWithinTimeRange(startTime: Date, endTime: Date, overshoot: boolean): Date[] {
		let months: Date[] = [];
		let d = new Date(startTime);
		d.setDate(1);
		d.setHours(0, 0, 0, 0);
		while (d < endTime) {
			months.push(new Date(d));
			d.setMonth(d.getMonth() + 1);
		}
		if (overshoot && months.length !== 0) {
			months.push(new Date(d));
		}
		return months;
	}

	formatTime(format: string, d: Date): string {
		let h = d.getHours();
		let minutes = zeroPad(d.getMinutes(), 2);
		let seconds = zeroPad(d.getSeconds(), 2);
		switch (format) {
			case "hh:mm:ss":
				return `${h}:${minutes}:${seconds}`;
			case "hh:mm":
				return `${h}:${minutes}`;
			case "hh":
				return `${h}`;
			case "dd":
				return `${d.getDate()}`;
			case "dd month":
				return `${d.getDate()} ${monthNamesShort[d.getMonth()]}`;
			case "month":
				return `${monthNamesShort[d.getMonth()]}`;
		}
		return "?";
	}

	renderTimeMarkers(cx: CanvasRenderingContext2D, canvasWidth: number, canvasHeight: number, startTimeMS: number, pixelsPerSecond: number) {
		let startS = startTimeMS / 1000;
		let endS = this.panTimeEndMS / 1000;

		let colors = ["rgba(255, 255, 255, 0.3)", "rgba(255, 255, 255, 0.5)"];
		let dpr = window.devicePixelRatio;

		let tryIntervals = [
			{ s: 10, f: 'hh:mm:ss' },
			{ s: 60, f: 'hh:mm' },
			{ s: 5 * 60, f: 'hh:mm' },
			{ s: 15 * 60, f: 'hh:mm' },
			{ s: 60 * 60, f: 'hh:mm' },
			{ s: 60 * 60, f: 'hh' },
			{ s: 24 * 60 * 60, f: 'dd month' },
			{ s: 24 * 60 * 60, f: 'dd' },
			{ s: 24 * 60 * 60 * 30, f: 'month' },
		];
		// Try different intervals until we get to one where our text can fit
		let intervalS = 0;
		let intervalFmt = '';
		let canvasWidthCss = canvasWidth / dpr;
		for (let interval of tryIntervals) {
			let txtWidth = interval.f.length;
			if (txtWidth * (endS - startS) / interval.s < canvasWidthCss * 0.15) {
				intervalS = interval.s;
				intervalFmt = interval.f;
				break;
			}
		}
		if (intervalS === 0) {
			intervalFmt = 'month';
		}

		let height = dpr * 4;

		let istart = 0;
		let iend = 0;
		let explicitDates: Date[] = [];

		let utcOffsetS = new Date().getTimezoneOffset() * 60;

		if (intervalFmt === "month") {
			// Month-based intervals, which are conceptually different
			explicitDates = this.computeMonthStartsWithinTimeRange(new Date(startS * 1000), new Date(endS * 1000), true);
			istart = 0;
			iend = explicitDates.length - 1;
		} else {
			// Second-based intervals (which can go all the way out to Days, but not beyond that)
			istart = Math.floor((startS - utcOffsetS) / intervalS);
			iend = Math.ceil((endS - utcOffsetS) / intervalS);
		}
		let showText = true;
		//console.log(istart, iend);
		for (let i = istart; i < iend; i++) {
			let evenodd = i % 2;
			let t1 = 0;
			let t2 = 0;
			if (explicitDates.length !== 0) {
				let d = explicitDates[i];
				let nextD = explicitDates[i + 1];
				t1 = this.timeMSToPixel(d.getTime(), canvasWidth, pixelsPerSecond);
				t2 = this.timeMSToPixel(nextD.getTime(), canvasWidth, pixelsPerSecond);
				if (intervalFmt === "month") {
					evenodd = d.getMonth() % 2;
				}
			} else {
				t1 = this.timeMSToPixel((i * intervalS + utcOffsetS) * 1000, canvasWidth, pixelsPerSecond);
				t2 = this.timeMSToPixel(((i + 1) * intervalS + utcOffsetS) * 1000, canvasWidth, pixelsPerSecond);
			}
			//console.log(t1, t2);
			cx.fillStyle = colors[evenodd];
			cx.fillRect(t1, canvasHeight - height, t2 - t1, canvasHeight);
			if (showText) {
				let d = new Date((i * intervalS + utcOffsetS) * 1000);
				cx.font = `${9 * dpr}px -apple-system, system-ui, sans-serif`;
				cx.textAlign = "center";
				cx.textBaseline = "bottom";
				cx.fillStyle = "rgba(255, 255, 255, 0.5)";
				let text = '';
				if (intervalFmt === "month") {
					d = explicitDates[i];
				}
				text = this.formatTime(intervalFmt, d);
				cx.fillText(text, t1, canvasHeight - height);
			}
		}
	}

	// Return the Y coordinate span of the timeline representing the given class of object (eg person, vehicle).
	// If the object class is not drawn, returns null
	tileTimelineSpan(detectedClass: string): { lineHeight: number, y: number } | null {
		let topY = 4.5;
		let lineHeight = 3 * window.devicePixelRatio;
		let idx = this.classes.indexOf(detectedClass);
		if (idx === -1) {
			return null
		} else {
			return { lineHeight, y: topY + idx * (lineHeight + 1) };
		}
	}

	renderTile(cx: CanvasRenderingContext2D, tile: EventTile, canvasWidth: number, pixelsPerSecond: number, videoStartTime?: Date) {
		let dpr = window.devicePixelRatio;
		let { x1: tx1, x2: tx2 } = this.renderedTileBounds(tile, canvasWidth, pixelsPerSecond);
		let bitWidth = (tx2 - tx1) / BitsPerTile;
		let debugMode = false; // Draw tile level and index onto canvas. Super useful when debugging tile creation bugs on the server.
		let boostThreshold = 4; // this is coupled to the sliding window size. If you make the window size bigger, you should make this bigger too.
		let boostLeft = 1.0 * dpr;
		let boostWidth = 1.75 * boostLeft; // should be about 2x boostLeft to keep the dot unbiased
		let bitWindowCount = new Uint8Array(BitsPerTile);
		let startBit = 0;
		let startX = tx1;
		if (videoStartTime) {
			// Don't render bits for events that occurred so long ago that the video has already been erased.
			// This is especially relevant for the zoomed-out high level tiles.
			let newStartX = this.timeMSToPixel(videoStartTime.getTime(), canvasWidth, pixelsPerSecond);
			startBit = Math.floor((newStartX - tx1) / bitWidth);
			startBit = Math.max(0, startBit);
			startX = tx1 + startBit * bitWidth;
		}
		for (let icls = 0; icls < this.classes.length; icls++) {
			cx.fillStyle = this.colors[icls];
			let bitmap = tile.classes[this.classes[icls]];
			if (bitmap) {
				SeekBarContext.countBitsInSlidingWindow(bitmap, bitWindowCount, 3);
				//console.log(this.cameraID, "window", bitWindowCount);
				let state = 0;
				let x1 = startX;
				let x2 = startX;
				let { lineHeight, y } = this.tileTimelineSpan(this.classes[icls])!;
				for (let bit = startBit; bit <= BitsPerTile; bit++) {
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
		}
		if (debugMode) {
			cx.strokeStyle = "rgba(255, 255, 255, 0.5)";
			cx.lineWidth = 1;
			cx.strokeRect(tx1, 0, tx2 - tx1, canvasWidth);
			cx.fillStyle = "rgba(255, 255, 255, 1)";
			cx.textAlign = "left";
			cx.textBaseline = "top";
			cx.fillText(`${tile.level}:${tile.tileIdx}`, tx1 + 4, 10);
		}
	}

	// Count the number of bits in a sliding window of windowSize size, and write that number
	// into bitWindowCount.
	// NOTE: The filter is not symmetrical. Should fix it.
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

	// Return a SeekBarTransform object for translating between pixel and time coordinates.
	transform(canvasEl: HTMLCanvasElement): SeekBarTransform {
		return SeekBarTransform.fromZoomLevelAndRightEdge(this.zoomLevel, this.panTimeEndMS, canvasEl.clientWidth * window.devicePixelRatio);
	}

	pixelsPerSecond(): number {
		return 1 / Math.pow(2, this.zoomLevel);
	}

	secondsPerPixel(): number {
		return Math.pow(2, this.zoomLevel);
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