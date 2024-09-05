import { CameraEvent } from "./events";

class EventCacheBucket {
	index: number; // Index of bucket (timeMS / bucketWidthMS)
	cameraID: number;
	events: CameraEvent[];
	fetchedAtMS: number;
	lastUsedMS: number;
	containsFutureEvents: boolean = false; // True if the end time of the bucket is not at least 5 seconds in the past

	constructor(index: number, bucketEndTime: Date, cameraID: number, events: CameraEvent[] = []) {
		let now = new Date();
		this.index = index;
		this.cameraID = cameraID;
		this.events = events;
		this.fetchedAtMS = now.getTime();
		this.lastUsedMS = now.getTime();
		this.containsFutureEvents = bucketEndTime.getTime() + 5000 > now.getTime();
	}

	key(): string {
		return `${this.cameraID}-${this.index}`;
	}
}

// Cache of camera events
// We break time down into discreet sections, and when fetching, we fetch those
// discreet buckets.
export class EventCache {
	bucketWidthMS = 5 * 60 * 1000; // Each bucket is 5 minutes
	maxBucketsInCache = 100;
	buckets: { [key: string]: EventCacheBucket } = {};
	isFetchInProgress: { [key: string]: boolean } = {};

	// Fetch all events in the given timespan
	async fetchEvents(cameraID: number, startTime: Date, endTime: Date): Promise<CameraEvent[]> {
		let startBucket = Math.floor(startTime.getTime() / this.bucketWidthMS);
		let endBucket = Math.floor(endTime.getTime() / this.bucketWidthMS);
		let outEvents: CameraEvent[] = [];
		let haveEvent = new Set<number>();
		for (let bucketIdx = startBucket; bucketIdx <= endBucket; bucketIdx++) {
			let bucket = this.getBucket(cameraID, bucketIdx);
			if (!bucket) {
				bucket = await this.fetchBucket(cameraID, bucketIdx);
			}
			for (let ev of bucket.events) {
				// We need to de-dup the events that we return, because the same event
				// can be present in more than one bucket. Also, we filter the results,
				// so that the caller gets only events that overlap the desired time range.
				if (!haveEvent.has(ev.id)) {
					outEvents.push(ev);
					haveEvent.add(ev.id);
				}
			}
		}
		return outEvents;
	}

	getBucket(cameraID: number, bucketIdx: number): EventCacheBucket | null {
		let key = `${cameraID}-${bucketIdx}`;
		let bucket = this.buckets[key];
		if (bucket === undefined) {
			return null;
		}
		bucket.lastUsedMS = new Date().getTime();
		return bucket;
	}

	async fetchBucket(cameraID: number, bucketIdx: number): Promise<EventCacheBucket> {
		this.autoEvict();
		let key = `${cameraID}-${bucketIdx}`;
		let startTime = new Date(bucketIdx * this.bucketWidthMS);
		let endTime = new Date((bucketIdx + 1) * this.bucketWidthMS);
		let events = await CameraEvent.fetchEvents(cameraID, startTime, endTime);
		let bucket = new EventCacheBucket(bucketIdx, endTime, cameraID, events);
		this.buckets[key] = bucket;
		return bucket;
	}

	autoEvict() {
		let keys = Object.keys(this.buckets);
		if (keys.length < this.maxBucketsInCache) {
			return;
		}
		keys.sort((a, b) => this.buckets[a].lastUsedMS - this.buckets[b].lastUsedMS);
		let nEvict = keys.length / 10;
		for (let i = 0; i < nEvict; i++) {
			delete this.buckets[keys[i]];
		}
	}
}

export let globalEventCache = new EventCache();