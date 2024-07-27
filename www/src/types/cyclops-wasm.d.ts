
declare module '@/wasm/cyclops-wasm.js' {
	interface CyclopsModule {
		_malloc(size: number): number;
		_free(ptr: number): void;
		_onoff_encode_3_max_output_size(number): number;
		_onoff_decode_3(input: number, input_byte_size: number, output: number, output_byte_size: number): number;
		HEAPU8: Uint8Array;
	}

	export default function createModule(): Promise<CyclopsModule>;
}
