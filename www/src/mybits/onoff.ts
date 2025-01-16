import { cyWasm } from "@/wasm/load";

// BinaryDecoder is a helper for decoding a binary stream containing various elements
export class BinaryDecoder {
	buffer: Uint8Array;
	pos = 0;

	constructor(buffer: Uint8Array, pos = 0) {
		this.buffer = buffer;
		this.pos = pos;
	}

	get remaining(): number {
		return this.buffer.length - this.pos;
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

export function onoffMaxOutputSize(inputBits: number): number {
	return cyWasm._onoff_encode_3_max_output_size(inputBits);
}

export function decodeOnoff(decoder: BinaryDecoder, encodedBufferLength: number, output: Uint8Array, expectedOutputBits: number) {
	let encodedBuffer = cyWasm._malloc(encodedBufferLength);
	let decodeBuffer = cyWasm._malloc(output.length);
	decoder.byteArray(cyWasm.HEAPU8, encodedBufferLength, encodedBuffer);
	let nDecodedBits = cyWasm._onoff_decode_3(encodedBuffer, encodedBufferLength, decodeBuffer, output.length);
	if (nDecodedBits !== expectedOutputBits) {
		throw new Error(`Expected ${expectedOutputBits} bits of output from onoff_decode_3, got ${nDecodedBits}`);
	}
	let nDecodedBytes = Math.ceil(nDecodedBits / 8);
	output.set(cyWasm.HEAPU8.subarray(decodeBuffer, decodeBuffer + nDecodedBytes));
	cyWasm._free(encodedBuffer);
	cyWasm._free(decodeBuffer);
}

// Returns the number of bytes written to encodedBuffer
export function encodeOnoff(input: Uint8Array, encodedBuffer: Uint8Array): number {
	let inputBuffer = cyWasm._malloc(input.length);
	let encodedBufferPtr = cyWasm._malloc(encodedBuffer.length);
	cyWasm.HEAPU8.set(input, inputBuffer);
	let nEncodedBytes = cyWasm._onoff_encode_3(inputBuffer, input.length * 8, encodedBufferPtr, encodedBuffer.length);
	encodedBuffer.set(cyWasm.HEAPU8.subarray(encodedBufferPtr, encodedBufferPtr + nEncodedBytes));
	cyWasm._free(inputBuffer);
	cyWasm._free(encodedBufferPtr);
	return nEncodedBytes;
}