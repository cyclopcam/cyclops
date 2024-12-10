import { CameraEvent } from "./events";

function bucketKey(cameraID: number, bucketIdx: number): string {
	return `${cameraID}-${bucketIdx}`;
}

type fetchCallback = () => void;

class EventCacheBucket {
	index: number; // Index of bucket (timeMS / bucketWidthMS)
	cameraID: number;
	events: CameraEvent[];
	fetchedAtMS: number;
	lastUsedMS: number;
	containsFutureEvents: boolean = false; // True if the end time of the bucket is not at least 5 seconds in the past
	onLoad: fetchCallback[] = [];

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
	// Each bucket is 10 minutes
	// 10 seems like the right number
	bucketWidthMS = 10 * 60 * 1000;
	maxBucketsInCache = 200;
	buckets: { [key: string]: EventCacheBucket } = {};
	busyFetching: { [key: string]: EventCacheBucket } = {};

	// Fetch all events in the given timespan
	fetchEvents(cameraID: number, startTimeMS: number, endTimeMS: number, allowFetch: boolean, onFetch?: fetchCallback): CameraEvent[] {
		let startBucket = Math.floor(startTimeMS / this.bucketWidthMS);
		let endBucket = Math.ceil(endTimeMS / this.bucketWidthMS);
		let outEvents: CameraEvent[] = [];
		let haveEvent = new Set<number>();
		let now = new Date().getTime();
		for (let bucketIdx = startBucket; bucketIdx < endBucket; bucketIdx++) {
			let bucket = this.getBucket(cameraID, bucketIdx);
			if (bucket && bucket.containsFutureEvents && now - bucket.fetchedAtMS > 5000) {
				// Invalidate realtime buckets every 5 seconds (but keep latest bucket around until the new one arrives)
				bucket = null;
			}
			if (!bucket) {
				let busy = this.busyFetching[bucketKey(cameraID, bucketIdx)];
				if (busy) {
					if (onFetch) {
						busy.onLoad.push(onFetch);
					}
				} else if (allowFetch) {
					this.fetchBucket(cameraID, bucketIdx, onFetch);
				}
				continue;
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
		let bucket = this.buckets[bucketKey(cameraID, bucketIdx)];
		if (bucket === undefined) {
			return null;
		}
		bucket.lastUsedMS = new Date().getTime();
		return bucket;
	}

	async fetchBucket(cameraID: number, bucketIdx: number, onLoad: fetchCallback | undefined) {
		this.autoEvict();
		let key = `${cameraID}-${bucketIdx}`;
		let startTime = new Date(bucketIdx * this.bucketWidthMS);
		let endTime = new Date((bucketIdx + 1) * this.bucketWidthMS);
		let bucket = new EventCacheBucket(bucketIdx, endTime, cameraID, []);
		if (onLoad) {
			bucket.onLoad.push(onLoad);
		}
		this.busyFetching[key] = bucket;
		try {
			bucket.events = await CameraEvent.fetchEvents(cameraID, startTime, endTime);
		} catch (e) {
			// TODO: show network failure
			console.error(`Failed to fetch events for ${key}: ${e}`);
		}
		delete this.busyFetching[key];
		this.buckets[key] = bucket;
		for (let cb of bucket.onLoad) {
			cb();
		}
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