# Channels

| Name    | Securable  | Locked |
|---------|------------|--------|
| BLE     | Yes        | Yes    |
| HTTP    | Yes (TLS)  | Yes    |
| Serial  | No (Wired) | Yes?   |
| Bridge  | Depends    | Yes    |
| Connect | Yes (TLS)  | Yes?   |

# Transports

| Name | Securable |
|------|-----------|
| MQTT | Yes (TLS) |
| OSC  | No        |

# Drivers

| Name  | Messaging | Endpoints | Managed | BLE | HTTP | Serial |
|-------|-----------|-----------|---------|-----|------|--------|
| Go    | Done?     | Done      | Done?   | Yes | Yes  | Yes    |
| TS    | Done?     | Done      | Done?   | Yes | Yes  | Yes    |
| Swift | Done?     | Done      | Done?   | Yes | Yes  |        |

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
- Swift: Add "Channel Width" concept
