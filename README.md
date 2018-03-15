# naos-esp

[![Build Status](https://travis-ci.org/shiftr-io/naos-esp.svg?branch=master)](https://travis-ci.org/shiftr-io/naos-esp)

**The [Networked Artifacts Operating System](https://github.com/shiftr-io/naos) component for the ESP platform.**

You can use this component in any [esp-idf](https://github.com/espressif/esp-idf) project, but using the [naos](https://github.com/shiftr-io/naos) command line utility is recommended.

## Standalone Requirements

- The [esp-mqtt](https://github.com/256dpi/esp-mqtt) component must be installed alongside `naos-esp`.
- Bluetooth must be enabled via `menuconfig`.
- Partitions should be configured as described [here](https://github.com/shiftr-io/naos-esp/blob/master/test/partitions.csv).

## Device Management Protocol

The following device management protocol is based on an "action down, data up" principle and tries to be as slim as possible to ease implementation.

### Discovery

Devices will subscribe to the global `naos/collect` topic and publish to the global `naos/announcement` topic if requested. The published data has the following format: `device_type,firmware_version,device_name,base_topic`.

### Heartbeat

Devices will periodically publish to the local `naos/heartbeat` topic. The published data has the following format: `device_type,firmware_version,device_name,free_heap,up_time,running_partition`.

### Configuration

Devices will subscribe to the local `naos/get/+`, `naos/set/+` and `naos/unset/+` topics to handle parameter read, write and delete requests. The device will acknowledge read and write actions by responding with the saved value on the local topic `naos/value/+`.

### Remote Logging

Devices will subscribe to the local `naos/record` topic and enable or disable remote logging when `on` or `off` is supplied
 respectively. The device will send log messages to the local `naos/log` topic if remote logging is activated.

### Firmware Update

Devices will subscribe to the local `naos/update/begin` topic and wait for an update request of the format `total_size`. If one is received the device will publish a request for the next chunk of data by publishing a `max_size` message to the local `naos/update/next` topic. The other end should then publish the next chunk of data to the local `naos/update/write` topic and wait for the next chunk request. When all data has been written the updater publishes a message to the local `naos/update/finish` topic to make the device commit the update and restart to the newly received firmware.

### Remote Debugging

Devices will subscribe to the local `naos/debug` topic and read the coredump from flash on every request and publish it to the local topic `naos/coredump`. If the payload of the request is set to `delete` the stored coredump will be removed after reading.
