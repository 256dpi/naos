# nadk

**the networked artifacts development kit**

## Requirements

- The [esp-mqtt](https://github.com/256dpi/esp-mqtt) components must be installed alongside the nadk.
- Bluetooth must be enabled via `menuconfig`.
- At least two OTA partitions must be configured via `menuconfig`.

## Device Management Protocol

### Discovery

All devices will subscribe to the global `nadk/collect` topic and publish to the global `nadk/announcement` topic if requested. The published data has the following format: `device_type,firmware_version,device_name,base_topic`.

### Heartbeat

All devices will periodically publish to the local `nadk/heartbeat` topic. The published data has the following format: `device_type,firmware_version,device_name,free_heap,up_time,running_partition`.

### Firmware Update

All devices will subscribe to the local `nadk/update/begin` topic and wait for an update request of the format `total_size`. If one is received the device will publish a request for the next chunk of data by publishing a `max_size` message to the local `nadk/update/next` topic. The other end will then publish the next chunk of data to the local `nadk/update/chunk` topic and wait for the next chunk request. When all data has been written the updater publishes a message to the local `nadk/update/finish` topic to make the device commit the update and restart to the newly received firmware.
