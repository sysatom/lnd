# LND - Linux Network Diagnoser

<p align="center">
  <img src="https://img.shields.io/badge/license-MIT-blue?style=flat-square" alt="License">
  <img src="https://img.shields.io/badge/platform-linux-lightgrey?style=flat-square" alt="Platform">
</p>

**LND** is a TUI-based Swiss Army knife for Linux network diagnostics. It integrates `ping`, `ethtool`, `netstat` and `/proc` analysis to help you pinpoint packet loss, retransmissions, and configuration issues in one place.

## Features
- **🩺 Kernel-level diagnostics**: real-time TCP retransmission rate calculation and monitoring of UDP buffer overflows.
- **🖥️ Deep environment insights**: detect NIC driver versions, offload features, and key sysctl parameters.
- **⚡ Real-time monitoring**: millisecond updates for bandwidth, packet loss, and latency jitter.
- **🌐 NAT & Connectivity**: STUN-based NAT type detection and multi-target connectivity probing.
- **📦 Ready to use**: single static binary, no dependencies, supports AMD64/ARM64.

## Installation
```bash
# Automatic install
curl -sfL https://github.com/sysatom/lnd/releases/latest/download/install.sh | sudo bash
```

## Usage
Requires root privileges to access low-level data:
```bash
sudo lnd
```

## Configuration

LND supports configuration via a YAML file. By default, it looks for `~/.lnd.yaml`.

You can also specify a config file using the `--config` flag:
```bash
sudo lnd --config /path/to/config.yaml
```

### Example Configuration

Ref. config.example.yaml
