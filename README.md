# naos

[![Test](https://github.com/256dpi/naos/actions/workflows/test.yml/badge.svg)](https://github.com/256dpi/naos/actions/workflows/test.yml)
[![GoDoc](https://godoc.org/github.com/256dpi/naos?status.svg)](http://godoc.org/github.com/256dpi/naos)
[![Release](https://img.shields.io/github/release/256dpi/naos.svg)](https://github.com/256dpi/naos/releases)

**The Networked Artifacts Operating System.**

The Networked Artifacts Operating System (NAOS) is an open source project with the aim to simplify the development for
the ESP32 micro-controller. It is based on Espressif's ESP-IDF development framework and can be used standalone or
added to existing IDF projects. The IDF component implements a fully-managed operation layer that provides Bluetooth based
configuration, Wi-Fi and MQTT connection management, remote parameter management, remote logging, remote debugging and
remote firmware updates. The several features are available through an open MQTT interface that can be easily integrated.

The additional NAOS command line utility implements a basic fleet management using the provided features. It can be used
to discover and monitor devices, manage parameters, access logs, download crash logs and perform over the air updates.
Furthermore, it drastically simplifies working with ESP-IDF by fully managing the project and its dependencies.
