---
title: Quickstart
---

# Quickstart

## Installation

First, you need to install the latest version of the `naos` command line utility (CLI):

```
curl -L https://naos.shiftr.io/install.sh | bash
```

*You can also download the binary manually from <https://github.com/shiftr-io/naos/releases> if you don't want to run the above shell script.*

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
