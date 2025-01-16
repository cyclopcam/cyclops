# WASM sources

To rebuild from source:

1. `cd pkg/mybits`
2. `./build-wasm`
3. To test: `node test-wasm.mjs`

## Why keep build artifacts in git?

The reason I'm keeping `cylops-wasm.js` and `cyclops-wasm.wasm` in git is
because building them requires installing docker, and that's something that I
don't want to enforce upon somebody building from source. Most developer setups
already have docker installed, so it's no issue there. But deployment devices
like a raspberry pi will often not have docker installed, and I don't want to
add that burden. If docker was always just an "apt-get" away, then it might be
appropriate to revisit this decision.
