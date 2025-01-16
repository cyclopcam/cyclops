import raw from "./wasm-bin/tester.mjs";
import { assert } from 'console';

function testDecode(m) {
	console.log("testDecode");
	let onoff_encode_3_max_output_size = m.cwrap("onoff_encode_3_max_output_size", 'number', ['number']);
	// expect to see '21' output in the console
	for (let i = 0; i < 5; i++) {
		console.log(`max_output_size(${i})`, onoff_encode_3_max_output_size(i));
	}

	// size_t onoff_decode_3(const unsigned char* input, size_t input_byte_size, unsigned char* output, size_t output_byte_size)
	let onoff_decode_3 = m.cwrap("onoff_decode_3", 'number', ['number', 'number', 'number', 'number']);

	// We generate this little test data sample with TestMakeTestPattern() in onoff_test.go

	// Encoded bytes: [32 182 85 23]
	let encodedBytes = new Uint8Array([32, 182, 85, 23]);
	let encodedBytesPtr = m._malloc(encodedBytes.length);
	m.HEAPU8.set(encodedBytes, encodedBytesPtr);

	// Expected decoded bytes: [3 255 255 255 255 255 7 127]

	// Allocate memory for decoded bytes
	let decodedBytesPtr = m._malloc(8);
	let resp = onoff_decode_3(encodedBytesPtr, encodedBytes.length, decodedBytesPtr, 8);
	console.log("decode response: ", resp);
	console.log("Decode bytes: ", m.HEAPU8.subarray(decodedBytesPtr, decodedBytesPtr + 8));
	assert(resp == 64); // returns number of bits
	assert(m.HEAPU8.subarray(decodedBytesPtr, decodedBytesPtr + 8).toString() == "3,255,255,255,255,255,7,127");

	m._free(encodedBytesPtr);
	m._free(decodedBytesPtr);
}

function testEncode(m) {
	console.log("testEncode");
	let onoff_encode_3 = m.cwrap("onoff_encode_3", 'number', ['number', 'number', 'number', 'number']);
	let onoff_decode_3 = m.cwrap("onoff_decode_3", 'number', ['number', 'number', 'number', 'number']);
	let onoff_encode_3_max_output_size = m.cwrap("onoff_encode_3_max_output_size", 'number', ['number']);

	let rawBytes = new Uint8Array([0xf0, 0x00, 0x03]);
	let rawBytesPtr = m._malloc(rawBytes.length);
	m.HEAPU8.set(rawBytes, rawBytesPtr);

	let encBytes = new Uint8Array(onoff_encode_3_max_output_size(rawBytes.length * 8));
	let encBytesPtr = m._malloc(encBytes.length);
	m.HEAPU8.set(encBytes, encBytesPtr);

	let encodedBytes = onoff_encode_3(rawBytesPtr, rawBytes.length * 8, encBytesPtr, encBytes.length);
	console.log("encoded bytes: ", encodedBytes);

	// Decode

	let rawBytes2 = new Uint8Array(rawBytes.length);
	let rawBytes2Ptr = m._malloc(rawBytes2.length);
	let decodedBits = onoff_decode_3(encBytesPtr, encodedBytes, rawBytes2Ptr, rawBytes2.length);
	console.log("decoded bits: ", decodedBits);
	// Copy decoded bytes to JS
	rawBytes2.set(m.HEAPU8.subarray(rawBytes2Ptr, rawBytes2Ptr + rawBytes2.length));
	console.log("original bytes: ", rawBytes);
	console.log("decoded bytes: ", rawBytes2);
	for (let i = 0; i < rawBytes.length; i++) {
		assert(rawBytes[i] == rawBytes2[i]);
	}
}

async function run() {
	let m = await raw();
	testDecode(m);
	testEncode(m);
}

run();
