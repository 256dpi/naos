# Channels

| Name   | Securable | Locked |
|--------|-----------|--------|
| BLE    | Yes (TLS) | Yes    |
| HTTP   | Yes (TLS) | Yes    |
| Bridge | Depends   | Yes    |

# Transports

| Name | Securable |
|------|-----------|
| MQTT | Yes (TLS) |
| OSC  | No        |

# Drivers

| Name  | Messaging | Endpoints | Managed | BLE | HTTP | OSC |
|-------|-----------|-----------|---------|-----|------|-----|
| Go    | Done?     | Done      | Done?   |     |      |     |
| TS    | Done?     | Done      | Done?   |     |      |     |
| Swift | Done?     | Done      | Done?   |     |      |     |

# Endpoints

| Name    | Go   | TS   | Swift |  
|---------|------|------|-------|  
| Params  | Done | Done | Done  |
| Update  | Done | Done | Done  | 
| FS      | Done | Done | Done  |
| Relay   |      | Done | Done  |
| Metrics | Done | Done | Done  |
| Auth    |      | Done |       |

# TODO

- Decouple Swift Messaging
- Align Swift Managed Device
- Make channels reference counted?
- Remove "256dpi/AsyncBluetooth" repo
