import { BinaryDecoder, decodeOnoff, encodeOnoff, onoffMaxOutputSize } from "@/mybits/onoff";

// DetectionZone is an arbitrarily sized bitmap where every bit indicates
// whether that portion of the image should be considered for motion/object detection.
export class DetectionZone {
	width: number;
	height: number;
	active: Uint8Array;

	constructor(width: number, height: number) {
		this.width = width;
		this.height = height;
		this.active = new Uint8Array(Math.ceil(width * height / 8));
	}

	static decodeBase64(b64: string): DetectionZone {
		let raw = Buffer.from(b64, "base64");
		let version = raw.readUint8(0);
		if (version !== 0) {
			throw new Error("Unknown DetectionZone version: " + version);
		}
		let width = raw.readUInt8(1);
		let height = raw.readUInt8(2);
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
		return Buffer.from(output.subarray(0, 3 + nBytes)).toString("base64");
	}

	clone(): DetectionZone {
		let dz = new DetectionZone(this.width, this.height);
		dz.active.set(this.active);
		return dz;
	}
}