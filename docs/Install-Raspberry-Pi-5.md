# Install OS

Use Raspberry Pi Imager to initialize a new SD card with "Raspberry Pi OS Lite
(64-bit)".

For Hailo:

> sudo apt update

> sudo apt upgrade

> sudo apt install hailofw hailort

> sudo reboot

Run raspi-config, and click Advanced Options, then PCIe Speed, and then enabled
PCIe Gen 3. Reboot.

# Build from source

    sudo apt install git libavformat-dev libswscale-dev ffmpeg cmake gcc g++ pkg-config libturbojpeg0-dev wireguard wireguard-tools
