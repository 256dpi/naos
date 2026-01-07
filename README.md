# NAOS

[![GoDoc](https://godoc.org/github.com/256dpi/naos?status.svg)](http://godoc.org/github.com/256dpi/naos)
[![Release](https://img.shields.io/github/release/256dpi/naos.svg)](https://github.com/256dpi/naos/releases)

**The Networked Artifacts Operating System.**

The Networked Artifacts Operating System (NAOS) is an open source project with the aim to simplify the development for the ESP32 microcontroller. It is based on Espressif's ESP-IDF development framework and can be used standalone or added to existing IDF projects. The IDF component implements a fully-managed operation layer that provides Bluetooth based configuration, Wi-Fi and MQTT connection management, remote parameter management, remote logging, remote debugging and remote firmware updates. The several features are available through an open MQTT interface that can be easily integrated.

The additional NAOS command line utility implements a basic fleet management using the provided features. It can be used to discover and monitor devices, manage parameters, access logs, download crash logs and perform over the air updates. Furthermore, it drastically simplifies working with ESP-IDF by fully managing the project and its dependencies.

## Quickstart

The following steps illustrate how you can get started with NAOS. We will guide you through installation, project setup and flashing.

### Installation

First, you need to install the latest version of the `naos` command line utility (CLI).

1. Download the binary from <https://github.com/256dpi/naos/releases>.
2. Move the binary to directory available through `$PATH`.

After the installation you can verify that `naos` is available:

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
