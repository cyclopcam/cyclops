const { assert } = require("console");
let m = require("./wasm-bin/mybits.js");
const { decode } = require("punycode");
m.onRuntimeInitialized = async () => {
	let onoff_encode_3_max_output_size = m.cwrap("onoff_encode_3_max_output_size", 'number', ['number']);
	// expect to see '21' output in the console
	console.log(onoff_encode_3_max_output_size(5));

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