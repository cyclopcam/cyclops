import * as base64 from "base64-arraybuffer";
import { BinaryDecoder, decodeOnoff, encodeOnoff, onoffMaxOutputSize } from "@/mybits/onoff";

// DetectionZone is an arbitrarily sized bitmap where every bit indicates
// whether that portion of the image should be considered for motion/object detection.
export class DetectionZone {
	width: number;
	height: number;
	active: Uint8Array;

	constructor(width: number, height: number) {
		if ((width & 7) !== 0) {
			throw new Error("DetectionZone width must be a multiple of 8");
		}
		this.width = width;
		this.height = height;
		this.active = new Uint8Array(width * height / 8);
	}

	static decodeBase64(b64: string): DetectionZone {
		let raw = new Uint8Array(base64.decode(b64));
		let version = raw[0];
		if (version !== 0) {
			throw new Error("Unknown DetectionZone version: " + version);
		}
		let width = raw[1];
		let height = raw[2];
		let decoder = new BinaryDecoder(raw, 3);
		let nBits = width * height;
		let dz = new DetectionZone(width, height);
		decodeOnoff(decoder, decoder.remaining, dz.active, nBits);
		return dz;
	}

	toBase64(): string {
		let output = new Uint8Array(3 + onoffMaxOutputSize(this.width * this.height));
		output[0] = 0;
		output[1] = this.width;
		output[2] = this.height;
		let nBytes = encodeOnoff(this.active, output.subarray(3));
		return base64.encode(output.subarray(0, 3 + nBytes));
	}

	clone(): DetectionZone {
		let dz = new DetectionZone(this.width, this.height);
		dz.active.set(this.active);
		return dz;
	}

	get(x: number, y: number): boolean {
		let i = y * this.width + x;
		return (this.active[i >> 3] & (1 << (i & 7))) !== 0;
	}

	set(x: number, y: number, value: boolean) {
		let i = y * this.width + x;
		if (value) {
			this.active[i >> 3] |= 1 << (i & 7);
		} else {
			this.active[i >> 3] &= ~(1 << (i & 7));
		}
	}

	fill(value: boolean) {
		this.active.fill(value ? 0xFF : 0);
	}
}