import { EventTile } from "./eventTile";

export class CachedEventTile {
	cameraID: number;
	tile: EventTile;
	lastUsedMS: number; // Used for cache eviction of least recently used
	fetchedAtMS: number; // Used to detect stale contemporary tiles (i.e. tiles that were still being created when they were fetched)

	constructor(cameraID: number, tile: EventTile) {
		this.cameraID = cameraID;
		this.tile = tile;
		let nowMS = new Date().getTime();
		this.lastUsedMS = nowMS;
		this.fetchedAtMS = nowMS;
	}

	get key(): string {
		return CachedEventTile.makeKey(this.cameraID, this.tile.level, this.tile.tileIdx);
	}

	static makeKey(cameraID: number, level: number, tileIdx: number): string {
		return `${cameraID}-${level}-${tileIdx}`;
	}
}

export type FetchCallback = (tile: CachedEventTile) => void;

export class EventTileCache {
	// Let's say average tile has 10 classes, and each class is 128 bytes (1024 bits).
	// 10 * 128 = 1280 bytes per tile.
	// 200 tiles = 200 * 1280 = 256000 bytes = 250 KB
	maxTiles = 200;
	tiles: { [key: string]: CachedEventTile } = {};
	cameraVideoStartTime: { [cameraID: number]: Date } = {}; // Oldest frame of video footage for this camera
	fetching = new Map<string, FetchCallback[]>();
	fetchCount = 0;
	maxStaleSeconds = 5; // Maximum amount of time that we'll allow a tile to be stale

	// Get a tile from the cache.
	// If afterFetch is null, then we don't fetch a missing tile.
	// If afterFetch is not null, then we fetch the tile if it's missing, and call afterFetch() on completion.
	getTile(cameraID: number, level: number, tileIdx: number, afterFetch?: FetchCallback): EventTile | undefined {
		let key = CachedEventTile.makeKey(cameraID, level, tileIdx);
		let tile = this.tiles[key];
		let fetchTile = afterFetch && !tile;
		if (tile) {
			// If the tile is stale, queue up a fetch, but return what we have in the meantime,
			// so that the renderer doesn't flicker when we're waiting for a fresh tile.
			fetchTile = afterFetch && this.isStale(tile);
			tile.lastUsedMS = new Date().getTime();
		}
		if (fetchTile) {
			let isFetching = this.fetching.has(key);
			if (afterFetch) {
				let f = this.fetching.get(key);
				if (f) {
					// 100 is just an arbitrary limit to the number of watchers. I can imagine easily
					// hitting that limit on zooming in/out, with a high latency connection to the server.
					// But the odds are extremely high that all the callbacks are pointing to the same function.
					if (f.length < 100) {
						f.push(afterFetch);
					}
				} else {
					this.fetching.set(key, [afterFetch]);
				}
			}
			if (!isFetching) {
				// We don't await for this. The caller is expected to use a callback "afterFetch" to get notified when the fetch finishes.
				this.fetchTile(cameraID, level, tileIdx);
			}
		}
		return tile?.tile;
	}

	async fetchTile(cameraID: number, level: number, tileIdx: number) {
		let key = CachedEventTile.makeKey(cameraID, level, tileIdx);
		try {
			// Set the fetch time to BEFORE we emit the request
			let fetchedAtMS = new Date().getTime();
			let fetchResult = await EventTile.fetchTiles(cameraID, level, tileIdx, tileIdx + 1);
			this.cameraVideoStartTime[cameraID] = fetchResult.videoStartTime;
			if (fetchResult.tiles.length !== 0) {
				let cachedTile = this.insertTile(cameraID, fetchResult.tiles[0]);
				cachedTile.fetchedAtMS = fetchedAtMS;
				this.fetchCount++;
				for (let callback of (this.fetching.get(key) || [])) {
					callback(cachedTile);
				}
			}
		} catch (e) {
			console.error(`Failed to fetch tile ${key}: ${e}`);
		}
		this.fetching.delete(key);
	}

	// Return true if the tile is stale (i.e. new events have possibly been recorded since we last fetched this tile)
	isStale(tile: CachedEventTile): boolean {
		let now = new Date().getTime();
		if (tile.tile.endTimeMS < now) {
			return false;
		}
		return now - tile.fetchedAtMS > this.maxStaleSeconds * 1000;
	}

	insertTile(cameraID: number, tile: EventTile): CachedEventTile {
		this.autoEvict();
		let key = CachedEventTile.makeKey(cameraID, tile.level, tile.tileIdx);
		let cached = new CachedEventTile(cameraID, tile);
		this.tiles[key] = cached;
		return cached;
	}

	autoEvict() {
		let keys = Object.keys(this.tiles);
		if (keys.length < this.maxTiles) {
			return
		}
		keys.sort((a, b) => this.tiles[a].lastUsedMS - this.tiles[b].lastUsedMS);
		let nEvict = keys.length / 10;
		for (let i = 0; i < nEvict; i++) {
			delete this.tiles[keys[i]];
		}
	}
}

export let globalTileCache = new EventTileCache();
