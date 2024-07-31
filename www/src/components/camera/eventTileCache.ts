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

export class EventTileCache {
	// Let's say average tile has 10 classes, and each class is 128 bytes (1024 bits).
	// 10 * 128 = 1280 bytes per tile.
	// 200 tiles = 200 * 1280 = 256000 bytes = 250 KB
	maxTiles = 200;
	tiles: { [key: string]: CachedEventTile } = {};
	fetching = new Set<string>();
	fetchCount = 0;

	// Get a tile from the cache.
	// If afterFetch is null, then we don't fetch a missing tile.
	// If afterFetch is not null, then we fetch the tile if it's missing, and call afterFetch() on completion.
	getTile(cameraID: number, level: number, tileIdx: number, afterFetch: ((tile: CachedEventTile) => void) | null): EventTile | undefined {
		let key = CachedEventTile.makeKey(cameraID, level, tileIdx);
		let tile = this.tiles[key];
		if (tile) {
			tile.lastUsedMS = new Date().getTime();
		}
		if (!tile && afterFetch && !this.fetching.has(key)) {
			this.fetchTile(cameraID, level, tileIdx, afterFetch);
		}
		return tile?.tile;
	}

	async fetchTile(cameraID: number, level: number, tileIdx: number, afterFetch: ((tile: CachedEventTile) => void) | null) {
		let key = CachedEventTile.makeKey(cameraID, level, tileIdx);
		this.fetching.add(key);
		try {
			let fetchedAtMS = new Date().getTime();
			let tiles = await EventTile.fetchTiles(cameraID, level, tileIdx, tileIdx + 1);
			let cachedTile = new CachedEventTile(cameraID, tiles[0]);
			cachedTile.fetchedAtMS = fetchedAtMS;
			this.tiles[key] = cachedTile;
			this.fetchCount++;
			if (afterFetch) {
				afterFetch(cachedTile);
			}
		} catch (e) {
			console.error(`Failed to fetch tile ${key}: ${e}`);
		}
		this.fetching.delete(key);
	}

	insertTile(cameraID: number, tile: EventTile) {
		this.autoEvict();
		let key = CachedEventTile.makeKey(cameraID, tile.level, tile.tileIdx);
		this.tiles[key] = new CachedEventTile(cameraID, tile);
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
