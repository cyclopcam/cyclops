# Hailo NN Accelerator

## Installation

Tested on a Raspberry Pi 5, running Raspberry Pi OS (their Debian), with a
Hailo8L M2 module, sold as the "Raspberry Pi AI kit".

`sudo apt install hailofw hailort`

After installing those dependencies, you should be able to run `build` in this
directory, and then Cyclops should pick up the Hailo accelerator when it start
up again. Note that you'll have to follow Hailo's instructions on getting it
setup, such as changing some settings in your raspi-config to enable the latest
Rpi5 firmware, and also enabling PCI Express v3.

## Batch Size Change Error

As of firmware/driver version 4.18.0, we can't change the batch size.
Specifically, if you create a device, then create a model with batch size 1,
then delete the model, and create another model with batch size 2, we get this
error:

> [HailoRT] [error] CHECK failed - Trying to configure a model with a batch=8
> bigger than internal_queue_size=4, which is not supported. Try using a smaller
> batch.

A workaround to this problem is to delete the device and recreate it. I've asked
on the forum: (post not available yet, waiting for moderators to approve).

An alternative workaround (which I intend to use) is to create all the models
up-front.

## Notes

1. The Hailo has hardware support for bilinear filtering. We should use that to
   scale the incoming image to the resolution of the NN, instead of cropping it,
   or blitting it onto a black canvas. The Hailo examples don't seem to indicate
   how to do this yet.
