# naos

[![Build Status](https://travis-ci.org/shiftr-io/naos.svg?branch=master)](https://travis-ci.org/shiftr-io/naos)
[![GoDoc](https://godoc.org/github.com/shiftr-io/naos?status.svg)](http://godoc.org/github.com/shiftr-io/naos)
[![Release](https://img.shields.io/github/release/shiftr-io/naos.svg)](https://github.com/shiftr-io/naos/releases)

**The Networked Artifacts Operating System.**

The Networked Artifacts Operating System (NAOS) is an open source project with the aim to simplify the development for
the ESP32 micro-controller. It is based on Espressif's ESP-IDF development framework and can be used standalone or
added to existing IDF projects. The IDF component implements a fully-managed operation layer that provides Bluetooth based
configuration, WiFi and MQTT connection management, remote parameter management, remote logging, remote debugging and
remote firmware updates. The several features are available through an open MQTT interface that can be easily integrated.

The additional NAOS command line utility implements a basic fleet management using the provided features. It can be used
to discover and monitor devices, manage parameters, access logs, download crash logs and perform over the air updates.
Furthermore, it drastically simplifies working with ESP-IDF by fully managing the project and its dependencies.
