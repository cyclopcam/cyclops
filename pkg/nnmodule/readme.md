# NN Module

This package dynamically loads a `.so` that implements neural network inference.
We do it like this so that our Go binary doesn't have a hard dependency on
whatever libraries are needed for, eg, the Hailo TPUs.
