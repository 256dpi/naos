# NAOS

[![GoDoc](https://godoc.org/github.com/256dpi/naos?status.svg)](http://godoc.org/github.com/256dpi/naos)
[![Release](https://img.shields.io/github/release/256dpi/naos.svg)](https://github.com/256dpi/naos/releases)

The **Networked Artifacts Operating System** (NAOS) is an open-source collection of protocols, libraries, and tools for building highly interactive smart devices with standardized remote management and tooling, primarily targeting the ESP32 family of microcontrollers.

At the core of NAOS is the **esp-idf component** that is used to enable and configure the various features in a firmware project. While the component can be used stand-alone, we recommend using the `naos` **build tool** to manage firmware projects.

The `naos-explorer` **TUI tool** and macOS **desktop app** are used to inspect, configure, and interact with NAOS-enabled devices over the supported connectivity standards. Groups of multiple NAOS devices can be managed with the `naos-fleet` **CLI tool**.

Finally, the **Go, TypeScript and Swift libraries** can be used to build web, mobile and desktop applications that interface with NAOS devices comfortably.

## Features

Out-of-the-box NAOS supports the following features:

- Configuration Parameters
- Multi-Dimensional Metrics
- File-System Access
- Remote Firmware Update
- Device Provisioning
- Sub-Device Relaying
- Password Protection
- Coredump Extraction
- Live Log Tailing

These features are provided by a messaging system exposed through various connectivity layers:

- WiFi (Link)
- Ethernet (Link)
- BLE (Channel)
- MQTT (Transport, Reverse-Channel)
- mDNS (Discovery)
- HTTP/WS-Server (Channel)
- HTTP/WS-Client (Reverse-Channel)
- Serial (Channel)
- OSC (Transport) [deprecated]
- Relay (Channel, Reverse-Channel)

*Link: physical/network interface · Channel: bidirectional session layer · Transport: routed or brokered messaging*

## Quickstart

The following steps illustrate how you can get started with NAOS. We will guide you through installation, project setup and flashing.

### Installation

First, you need to install the latest version of the `naos` command line utility (CLI).

1. Download the binary from <https://github.com/256dpi/naos/releases>.
2. Move the binary to a directory available in `$PATH`.

After installation, you can verify that `naos` is available:

```
naos help
```

### Project Setup

Create a new project in an empty directory somewhere on your computer:

```
naos create
```

*The CLI will create a `naos.json` configuration and a basic `src/main.c` file.*

Download and install all dependencies (this may take a few minutes):

```
naos install
```

*You can run `naos install` again to update the dependencies.*

Run the firmware on the connected ESP32 board:

```
naos run
```

*This will run the `build`, `flash` and `attach` commands in sequence.*

## License

Copyright © Joël Gähwiler.

Licensed under the Apache License, Version 2.0. See the [LICENSE](LICENSE) file for details.
