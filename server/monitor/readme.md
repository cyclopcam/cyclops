# 'Genuine' field

In `TrackedObject` we have a `Genuine` field. This is an attempt at reducing
false positives from the NN. We should aim to get rid of this kind of filtering,
and rely solely on the neural network. End to end is always the goal!

## Validation model

It's clear that I need some kind of more accurate validation model to prevent
false positives. In test case `testdata/tracking/0012-LD.mp4`, the first high
res (640x480) model that I found that does not make a false positive mistake is
NCNN 640x480 yolov8l.

Table of model inference times on NCNN (times in milliseconds). 640x480.

| Model   | RPi5 |
| ------- | ---- |
| yolov8m | 580  |
| yolov8l | 1114 |
