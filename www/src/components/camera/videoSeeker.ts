// VideoSeeker manages the process of seeking around inside a video timeline.
// Our cameras often have long keyframe intervals (eg 1 to 3 seconds), so we need
// to think about things like storing decoded frames in order to seek around inside
// delta frames.
export class VideoSeeker {
	cameraID = 0;

	constructor(cameraID: number) {
		this.cameraID = cameraID;
	}

	seekTo(timeMS: number) {
	}
}