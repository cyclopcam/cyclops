# NN Accelerator

This package dynamically loads a `.so` that implements neural network inference.
We do it like this so that our Go binary doesn't have a hard dependency on
whatever libraries are needed for, eg, the Hailo NPUs.

If there was a way to avoid this dynamic binary loading infrastructure, I would do it!

## Caveats

The Hailo models require the NMS thresholds to be set at compile time. Our NN
interface allows one to specify it at runtime. This is a problem, and will cause
confusion. I haven't decided how to deal with this yet.