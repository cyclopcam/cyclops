#!/bin/bash

# This script installs Cyclops from the internet (files.cyclopcam.org)

set -e

# Determine if sudo is needed
if [ "$(id -u)" -eq 0 ]; then
    SUDO=""
else
    SUDO="sudo"
    if ! command -v sudo >/dev/null 2>&1; then
        echo "Error: sudo is required but not installed."
        exit 1
    fi
fi

# Detect OS and architecture
OS=""
ARCH=$(uname -m)

case "$ARCH" in
    x86_64)
        ARCH="amd64"
        ;;
    aarch64)
        ARCH="arm64"
        ;;
    *)
        echo "Error: Unsupported architecture: $ARCH. Supported architectures are amd64 and arm64."
        exit 1
        ;;
esac

if [ -f /etc/os-release ]; then
    . /etc/os-release
    case "$VERSION_CODENAME" in
        jammy|noble|bookworm)
            OS="$VERSION_CODENAME"
            ;;
        *)
            echo "Error: Unsupported OS: $VERSION_CODENAME. Supported OS versions are jammy, noble, and bookworm."
            exit 1
            ;;
    esac
else
    echo "Error: Cannot determine OS version."
    exit 1
fi

# Install dependencies
$SUDO apt-get update
$SUDO apt-get install -y apt-transport-https ca-certificates curl gnupg

# Create keyring directory
$SUDO mkdir -p /etc/apt/keyrings

# Download and install the signing key
curl -fsSL https://files.cyclopcam.org/cyclopcam.gpg | $SUDO gpg --dearmor -o /etc/apt/keyrings/cyclopcam.gpg
if [ $? -ne 0 ]; then
    echo "Error: Failed to download or process the signing key."
    exit 1
fi

# Add the repository
# For bookworm/rpi5, it's crucial that we add the arch=arm64, otherwise it rejects us because we don't have an armhf package.
echo "deb [arch=$ARCH signed-by=/etc/apt/keyrings/cyclopcam.gpg] https://files.cyclopcam.org $OS main" | $SUDO tee /etc/apt/sources.list.d/cyclopcam.list
if [ $? -ne 0 ]; then
    echo "Error: Failed to add the repository."
    exit 1
fi

# Update and install cyclops
$SUDO apt-get update
$SUDO apt-get install -y cyclops

echo "Cyclops has been successfully installed."