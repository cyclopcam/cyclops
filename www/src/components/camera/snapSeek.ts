import type { CameraInfo } from "@/camera/camera";
import { globalEventCache } from "./eventCache";

export class SnapSeekState {
	// If this is not zero, this is the time where the snap icon should be shown.
	// If the appropriate circumstances come to pass (eg delay with no cursor movement),
	// then we will play the reel surrounding this point in time.
	posMS = 0;
	detectedClass = "";

	clear() {
		this.posMS = 0;
	}
}

// SnapSeek maintains the state of our snap-to-event feature.
// This allows the user to move the seek bar around on the video when zoomed out
// far, and we will automatically play a few seconds of interesting content
// close to the seek region.
export class SnapSeek {
	camera: CameraInfo;

	state = new SnapSeekState();

	constructor(camera: CameraInfo, state?: SnapSeekState) {
		this.camera = camera;
		if (state) {
			this.state = state;
		}
	}

	clear() {
		this.state.clear();
	}

	// Returns true if we found an event to snap to.
	snapSeekTo(seekTimeMS: number, maxSnapDistanceMS: number, allowFetch: boolean, onFetch?: () => void): boolean {
		let events = globalEventCache.fetchEvents(this.camera.id, seekTimeMS - maxSnapDistanceMS, seekTimeMS + maxSnapDistanceMS, allowFetch, onFetch);
		this.state.posMS = 0;
		this.state.detectedClass = "";

		let success = false;

		let closestMS = maxSnapDistanceMS;
		for (let ev of events) {
			for (let obj of ev.objects) {
				let eventCenterMS = (obj.startTimeMS() + obj.endTimeMS()) / 2;
				let deltaMS = Math.abs(eventCenterMS - seekTimeMS);
				if (deltaMS < closestMS) {
					success = true;
					closestMS = deltaMS;
					this.state.posMS = eventCenterMS;
					this.state.detectedClass = obj.cls;
				}
			}
		}
		//console.log("snap", this.state.posMS, this.state.detectedClass);

		return success;
	}
}
