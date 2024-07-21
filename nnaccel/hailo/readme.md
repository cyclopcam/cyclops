# Hailo NN Accelerator

## Installation

Tested on a Raspberry Pi 5, running Raspberry Pi OS (their Debian), with a Hailo8L M2 module,
sold as the "Raspberry Pi AI kit".

`sudo apt install hailofw hailort`

After installing those dependencies, you should be able to run `build` in this directory,
and then Cyclops should pick up the Hailo accelerator when it start up again. Note that
you'll have to follow Hailo's instructions on getting it setup, such as changing some
settings in your raspi-config to enable the latest Rpi5 firmware, and also enabling
PCI Express v3.

## Notes

There are various things we could do better here:

1. The Hailo has hardware support for bilinear filtering. We should use that to scale the incoming image
	to the resolution of the NN, instead of cropping it, or blitting it onto a black canvas. The Hailo
	examples don't seem to indicate how to do this yet.
2. Use a batch size of 8 (or anything larger than 1).
3. We should make sure that we can reach the same peak FPS that the official Hailo benchmarks achieve.
4. Use better models. Right now we're using yolov8s, but we could probably use yolov8m or yolov8l if we
	tried.