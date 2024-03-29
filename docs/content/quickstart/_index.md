---
title: Quickstart
---

# Quickstart

The following steps illustrate how you can get started with NAOS. We will guide you through installation, project setup and flashing.

## Installation

First, you need to install the latest version of the `naos` command line utility (CLI).

1. Download the binary from <https://github.com/256dpi/naos/releases>.
2. Move the binary to directory available through `$PATH`.

After the installation you can verify that `naos` is available:

```
naos help
```

## Project Setup

Create a new project in an empty directory somewhere on your computer:

```
naos create
```

*The CLI will create a `naos.json` configuration and an empty `src/main.c`  file.*

Download and install all dependencies (this may take a few minutes):

```
naos install
```

*You can run `naos install` anytime to update to the dependencies.*

Run the firmware on the connected ESP32:

```
naos run
```
