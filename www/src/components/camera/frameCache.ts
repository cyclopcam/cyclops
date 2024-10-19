import type { CameraInfo, Resolution, StreamInfo } from "@/camera/camera";
import type { AnalysisState } from "@/camera/nn";

export class CachedFrame {
	key: string;
	blob: Blob;
	lastUsed: number;
	frameTimeUnixMS: number;
	analysis?: AnalysisState;

	constructor(key: string, blob: Blob, frameTimeUnixMS: number, analysis?: AnalysisState) {
		this.key = key;
		this.blob = blob;
		this.lastUsed = Date.now();
		this.frameTimeUnixMS = frameTimeUnixMS;
		this.analysis = analysis;
	}
}

// One of these for LD and HD each
class frameCacheStreamInfo {
	fps: number; // from static stream info
	keyframeInterval: number; // from static stream info
	keyAnchorUnixMS: number; // from real frames

	constructor(info?: StreamInfo) {
		this.fps = info?.fps ?? 0;
		this.keyframeInterval = info?.keyframeInterval ?? 0;
		this.keyAnchorUnixMS = 0;
	}
}

export class FrameCache {
	frames: Map<string, CachedFrame> = new Map();
	maxSize = 2 * 1024 * 1024;
	currentSize = 0;
	streams = { ld: new frameCacheStreamInfo(), hd: new frameCacheStreamInfo() };

	static makeKey(resolution: string, timeMS: number) {
		return `${resolution}-${Math.round(timeMS)}`;
	}

	constructor(camera: CameraInfo) {
		this.streams.ld = new frameCacheStreamInfo(camera.ld);
		this.streams.hd = new frameCacheStreamInfo(camera.hd);
	}

	clear() {
		this.frames.clear();
		this.currentSize = 0;
	}

	// Notify the cache of the exact PTS of a keyframe, so that it has something to anchor
	// all subsequent keyframe requests to.
	addKeyframeTime(resolution: Resolution, timeMS: number) {
		this.streams[resolution].keyAnchorUnixMS = timeMS;
	}

	add(key: string, blob: Blob, frameTimeUnixMS: number, analysis?: AnalysisState): CachedFrame {
		const frame = new CachedFrame(key, blob, frameTimeUnixMS, analysis);
		if (this.frames.get(key)) {
			// In the unexpected case that we're replacing a frame
			this.currentSize -= this.frames.get(key)!.blob.size;
		}
		this.frames.set(key, frame);
		this.currentSize += blob.size;
		this.trim();
		return frame;
	}

	get(key: string): CachedFrame | undefined {
		const frame = this.frames.get(key);
		if (frame) {
			frame.lastUsed = Date.now();
		}
		return frame;
	}

	trim() {
		if (this.currentSize <= this.maxSize) {
			return;
		}
		let threshold = this.maxSize * 0.95;
		let sorted = [...this.frames.values()].sort((a, b) => b.lastUsed - a.lastUsed);
		while (this.currentSize > threshold && frames.length > 0) {
			let frame = sorted.pop()!;
			this.frames.delete(frame.key);
			this.currentSize -= frame.blob.size;
		}
	}

	//estimatedFPS(): { fps: number, randomFrameTimeMicro: number } {
	//	for (let frame of this.frames.values()) {
	//		if (frame.estimatedFPS !== 0) {
	//			return { fps: frame.estimatedFPS, randomFrameTimeMicro: frame.frameTimeMicro };
	//		}
	//	}
	//	return { fps: 0, randomFrameTimeMicro: 0 };
	//}

	// Snap to the nearest frame time, or nearest keyframe time.
	// Input and output are unix milliseconds
	snapToNearestFrame(timeMS: number, resolution: Resolution, keyFrame: boolean): number {
		let info = this.streams[resolution];
		if (info.fps === 0 || info.keyAnchorUnixMS === 0) {
			return timeMS;
		}
		let fps = info.fps;
		let keyframeInterval = info.keyframeInterval;
		let anchorMS = info.keyAnchorUnixMS;
		let msPerFrame = 1000 / fps;
		let snapMS = keyFrame ? keyframeInterval * msPerFrame : msPerFrame;
		let deltaFrames = (timeMS - anchorMS) / snapMS;
		deltaFrames = Math.round(deltaFrames);
		let newFrameTimeMS = anchorMS + (deltaFrames * snapMS);
		let result = Math.round(newFrameTimeMS);
		//console.log(`Snapped ${timeMS} to ${result} for ${resolution} ${fps}fps ${keyFrame ? "keyframe" : "frame"}`);
		return result;
	}
}