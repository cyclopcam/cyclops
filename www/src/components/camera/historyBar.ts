
import { EventTile } from "./eventTile";

const BaseSecondsPerTile = 1024;

// HistoryBar draws the lines at the bottom of a video which show the moments
// of interest when particular things were detected. For example, the bar might
// be white everywhere, but red where a person was detected.
export class HistoryBar {

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