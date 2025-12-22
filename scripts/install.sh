#!/bin/bash
set -e

REPO="yourusername/lnd"
BINARY="lnd"
INSTALL_DIR="/usr/local/bin"

echo "Detecting architecture..."
ARCH=$(uname -m)
case $ARCH in
    x86_64) ARCH="amd64" ;;
    aarch64) ARCH="arm64" ;;
    *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
if [ "$OS" != "linux" ]; then
    echo "Only Linux is supported."
    exit 1
fi

echo "Fetching latest release..."
LATEST_URL=$(curl -s https://api.github.com/repos/$REPO/releases/latest | grep "browser_download_url" | grep "$OS" | grep "$ARCH" | cut -d '"' -f 4)

if [ -z "$LATEST_URL" ]; then
    echo "Could not find a release for $OS/$ARCH"
    exit 1
fi

echo "Downloading $LATEST_URL..."
curl -L -o /tmp/lnd.tar.gz "$LATEST_URL"

echo "Installing to $INSTALL_DIR..."
tar -xzf /tmp/lnd.tar.gz -C /tmp
sudo mv /tmp/$BINARY $INSTALL_DIR/
sudo chmod +x $INSTALL_DIR/$BINARY

echo "Done! Run 'sudo lnd' to start."
