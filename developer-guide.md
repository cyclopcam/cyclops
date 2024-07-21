# Cyclops Developer Guide

## Neural Network Inference

We have an annoying inconsistency between Hailo accelerated NN inference and NCNN inference.
The NCNN interface can accept detection thresholds on each run, but the Hailo model needs
these parameters defined up-front. I have not yet decided how to get rid of this inconsistency.