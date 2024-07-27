
import { clamp, dateTime } from "@/util/util";

// HistoryBar draws the lines at the bottom of a video which show the moments
// of interest when particular things were detected. For example, the bar might
// be white everywhere, but red where a person was detected.
export class HistoryBar {

	static downloadTiles(canvasWidthPx: number, startTime: Date, endTime: Date) {
		let numSeconds = (endTime.getTime() - startTime.getTime()) / 1000;
		numSeconds = Math.max(numSeconds, 1);
		if (canvasWidthPx <= 1) {
			return;
		}
		// The tile API wants a level and start and end tile indices, so we need to do that conversion here.
		// Let's start.
		// At level zero, tiles are 1 second per pixel.
		// We figure out the right level, and then we fetch tiles that span our desired time range.
		let pixelsPerSecond = canvasWidthPx / numSeconds;
		let level = Math.floor(Math.log2(pixelsPerSecond));
		level = Math.max(level, 0);
	}
}