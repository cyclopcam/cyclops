
declare module '@/wasm/cyclops-wasm.js' {
	interface CyclopsModule {
		_malloc(size: number): number;
		_free(ptr: number): void;
		_onoff_encode_3_max_output_size(input_bit_size: number): number; // Returns maximum number of bytes output
		_onoff_decode_3(input: number, input_byte_size: number, output: number, output_byte_size: number): number; // Returns number of bits output
		_onoff_encode_3(input: number, input_bit_size: number, output: number, output_byte_size: number): number; // Returns number of bytes output
		HEAPU8: Uint8Array;
	}

	export default function createModule(): Promise<CyclopsModule>;
}
