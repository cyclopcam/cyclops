import { encodeQuery, fetchOrErr } from "@/util/util";
import { cyWasm } from "@/wasm/load";
import * as base64 from "base64-arraybuffer";

export const BaseSecondsPerTile = 1024; // Expect this to be a multiple of BitsPerTile. I went with 1:1 for simplicity.
export const BitsPerTile = 1024;

// SYNC-MAX-TILE-LEVEL
export const MaxTileLevel = 13;

export class BinaryDecoder {
	buffer: Uint8Array;
	pos = 0;

	constructor(buffer: Uint8Array) {
		this.buffer = buffer;
	}

	uvariant(): number {
		let result = 0;
		let shift = 0;
		let byte;
		do {
			byte = this.buffer[this.pos++];
			result |= (byte & 0x7f) << shift;
			shift += 7;
		} while (byte & 0x80);
		return result;
	}

	byte(): number {
		return this.buffer[this.pos++];
	}

	// Read into 'dst', at the destination offset provided.
	// If 'length' is not provided, read dst.length bytes
	// If 'dstOffset' is not provided, read into the start of 'dst'
	byteArray(dst: Uint8Array, length?: number, dstOffset?: number) {
		if (length === undefined) {
			length = dst.length;
		}
		dst.set(this.buffer.subarray(this.pos, this.pos + length), dstOffset);
		this.pos += length;
	}
}

// Map from class ID to 1024-bit event tile bitmap
type RawTileData = { [classID: number]: Uint8Array };

// SYNC-EVENT-TILE-JSON
interface EventTileJSON {
	camera: number;
	level: number;
	start: number; // rename to tileIdx?
	tile: string; // base64 encoded
}

// SYNC-GET-TILES-JSON
interface GetTileJSON {
	tiles: EventTileJSON[];
	idToString: { [key: number]: string };
	videoStartTime: number; // This doesn't vary with the tiles being fetched, but it's useful data to side-load
}

export interface FetchTilesResult {
	videoStartTime: Date;
	tiles: EventTile[];
}

export class EventTile {
	level = 0; // level 0 = 1 second per bit/pixel, level 1 = 2 seconds per bit/pixel, etc.
	tileIdx = 0; // unix second of start of tile / (1024 << level)
	// Map from class name (eg 'person', 'car') to a 1024 bit (128 byte) bitmap which
	// represents the presence of that object at that particular time point.
	classes: { [key: string]: Uint8Array } = {};

	// Uniquely identifying key for this tile
	get key(): string {
		return `${this.level}-${this.tileIdx}`;
	}

	get startTimeMS(): number {
		return this.tileIdx * ((1000 * BaseSecondsPerTile) << this.level);
	}

	get endTimeMS(): number {
		return (this.tileIdx + 1) * ((1000 * BaseSecondsPerTile) << this.level);
	}

	static getBit(bitmap: Uint8Array, bit: number): number {
		let byteIdx = bit >> 3;
		let bitIdx = bit & 7;
		return (bitmap[byteIdx] & (1 << bitIdx)) ? 1 : 0;
	}

	static async fetchTiles(camera: number, level: number, startIdx: number, endIdx: number): Promise<FetchTilesResult> {
		let query = {
			camera: camera,
			level: level,
			startIdx: startIdx,
			endIdx: endIdx,
		}
		let r = await fetchOrErr('/api/events/tiles?' + encodeQuery(query));
		if (!r.ok) {
			throw new Error(`Failed to fetch tiles: ${r.error}`);
		}
		let j = (await r.r.json()) as GetTileJSON;
		return {
			videoStartTime: new Date(j.videoStartTime),
			tiles: j.tiles.map(tj => EventTile.fromJSON(tj, j.idToString))
		}
	}

	static fromJSON(tj: EventTileJSON, idToString: { [key: number]: string }): EventTile {
		let tile = new EventTile();
		tile.level = tj.level;
		tile.tileIdx = tj.start;
		let buffer = new Uint8Array(base64.decode(tj.tile));
		return EventTile.decode(tile.level, tile.tileIdx, buffer, idToString);
	}

	static decode(level: number, tileIdx: number, buffer: Uint8Array, idToString: { [key: number]: string }): EventTile {
		let raw = EventTile.decodeRaw(buffer);
		let t = new EventTile();
		t.level = level;
		t.tileIdx = tileIdx;
		for (let classID in raw) {
			let className = idToString[parseInt(classID)];
			if (className === undefined) {
				console.warn(`Unknown class ID ${classID} in tile`);
				continue;
			}
			t.classes[className] = raw[classID];
		}
		return t;
	}

	static decodeRaw(buffer: Uint8Array): RawTileData {
		let decoder = new BinaryDecoder(buffer);
		let version = decoder.uvariant();
		if (version !== 1) {
			throw new Error(`Unknown event tile version ${version}`);
		}

		let tile: RawTileData = {};

		// Read the list of classes
		let numClasses = decoder.uvariant();
		let idxToClass: number[] = [];
		for (let i = 0; i < numClasses; i++) {
			let classID = decoder.uvariant();
			idxToClass.push(classID);
			tile[classID] = new Uint8Array(128);
		}

		// Read the 1024-bit lines of each class
		for (let cls of idxToClass) {
			// The first byte of the line is the encoded length.
			// If 128, then the line is raw.
			// If less than 128, then the line is encoded with our "onoff" encoding
			let encodedLength = decoder.byte();
			if (encodedLength === 128) {
				decoder.byteArray(tile[cls]);
			} else {
				EventTile.decodeOnoff(decoder, encodedLength, tile[cls]);
			}
		}

		return tile;
	}

	static decodeOnoff(decoder: BinaryDecoder, encodedBufferLength: number, output: Uint8Array) {
		let encodedBuffer = cyWasm._malloc(encodedBufferLength);
		let decodeBuffer = cyWasm._malloc(output.length);
		decoder.byteArray(cyWasm.HEAPU8, encodedBufferLength, encodedBuffer);
		let nDecodedBits = cyWasm._onoff_decode_3(encodedBuffer, encodedBufferLength, decodeBuffer, output.length);
		output.set(cyWasm.HEAPU8.subarray(decodeBuffer, decodeBuffer + output.length));
		cyWasm._free(encodedBuffer);
		cyWasm._free(decodeBuffer);
		if (nDecodedBits !== BitsPerTile) {
			throw new Error(`Expected ${BitsPerTile} bits of output from onoff_decode_3, got ${nDecodedBits}`);
		}
	}
}