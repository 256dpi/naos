---
title: Quickstart
---

# Quickstart

Install the NAOS command line utility:

```
curl github.com/run-install-script.sh
```

Create a new project:

```
mkdir project; cd project
naos create
```

Ensure the following files have been created:

```
naos.json
src/main.c
```

Download and install all dependencies:

```
naos install
```

Run the firmware on the connected ESP32:

```
naos run
```
