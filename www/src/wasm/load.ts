import cyclopsWasmModule, { type CyclopsModule } from '@/wasm/cyclops-wasm.js';

export let cyWasm: CyclopsModule;

export async function loadWASM() {
	cyWasm = await cyclopsWasmModule();
	//testOnoffDecode();
}

function testOnoffDecode() {
	console.log(cyWasm._onoff_encode_3_max_output_size(5));

	let encodedBytes = new Uint8Array([32, 182, 85, 23]);
	let encodedBytesPtr = cyWasm._malloc(encodedBytes.length);
	cyWasm.HEAPU8.set(encodedBytes, encodedBytesPtr);

	// Allocate memory for decoded bytes
	let decodedBytesPtr = cyWasm._malloc(8);
	let resp = cyWasm._onoff_decode_3(encodedBytesPtr, encodedBytes.length, decodedBytesPtr, 8);
	// Check the output here and make sure it's equal to the values from the comments below
	console.log("decode response: ", resp);
	console.log("Decode bytes: ", cyWasm.HEAPU8.subarray(decodedBytesPtr, decodedBytesPtr + 8));
	//assert(resp == 64); // returns number of bits
	//assert(m.HEAPU8.subarray(decodedBytesPtr, decodedBytesPtr + 8).toString() == "3,255,255,255,255,255,7,127");

	cyWasm._free(encodedBytesPtr);
	cyWasm._free(decodedBytesPtr);
}