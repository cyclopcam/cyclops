# mybits

This is a collection of functions for bit-manipulative stuff.

The most important thing in here is the encoder and decoder for our event tile
bitmaps. This is documented at the top of onoff.c

In addition, we compile the onoff decoder into a WASM module.

## Test/debug the native C code

See debug/test-bits.c and debug/test-onoff.c

## Test the Go interface

go test .

## Build and test the WASM code

`./build-wasm && node test-wasm.mjs`
