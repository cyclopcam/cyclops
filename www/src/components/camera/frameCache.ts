class CachedFrame {
	key: string;
	blob: Blob;
	lastUsed: number;
	estimatedFPS: number; // 0 if none available
	frameTimeMicro: number; // unix microseconds

	constructor(key: string, blob: Blob, estimatedFPS: number, frameTimeMicro: number) {
		this.key = key;
		this.blob = blob;
		this.lastUsed = Date.now();
		this.estimatedFPS = estimatedFPS;
		this.frameTimeMicro = frameTimeMicro;
	}
}

export class FrameCache {
	frames: Map<string, CachedFrame> = new Map();
	maxSize = 2 * 1024 * 1024;
	currentSize = 0;

	static makeKey(cameraID: number, resolution: string, timeMS: number) {
		return `${cameraID}-${resolution}-${timeMS}`;
	}

	clear() {
		this.frames.clear();
		this.currentSize = 0;
	}

	add(key: string, blob: Blob, estimatedFPS: number, frameTime: number) {
		const frame = new CachedFrame(key, blob, estimatedFPS, frameTime);
		if (this.frames.get(key)) {
			this.currentSize -= this.frames.get(key)!.blob.size;
		}
		this.frames.set(key, frame);
		this.currentSize += blob.size;
		this.trim();
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

	estimatedFPS(): { fps: number, randomFrameTimeMicro: number } {
		for (let frame of this.frames.values()) {
			if (frame.estimatedFPS !== 0) {
				return { fps: frame.estimatedFPS, randomFrameTimeMicro: frame.frameTimeMicro };
			}
		}
		return { fps: 0, randomFrameTimeMicro: 0 };
	}

	suggestNearestFrameTime(timeMS: number): number {
		let { fps, randomFrameTimeMicro } = this.estimatedFPS();
		if (fps === 0) {
			// maximum that we'd likely see
			fps = 30;
		}
		let frameInterval = 1 / fps;
		let microDelta = (timeMS * 1000) - randomFrameTimeMicro;
		let deltaFrames = (microDelta / 1000000) / frameInterval;
		deltaFrames = Math.round(deltaFrames);
		let newFrameTimeMicro = randomFrameTimeMicro + (deltaFrames * frameInterval * 1000000);
		console.log("estimated FPS", fps);
		return Math.round(newFrameTimeMicro / 1000);
	}
}