# Channels

| Name    | Securable  | Go  | TS  | Swift | 
|---------|------------|-----|-----|-------|
| BLE     | Yes        | Yes | Yes | Yes   |
| HTTP    | Yes (TLS)  | Yes | Yes | Yes   |
| Serial  | No (Wired) | Yes | Yes | Yes   |
| Bridge  | Depends    |     |     |       |
| Connect | Yes (TLS)  |     |     |       |

# Transports

| Name | Securable | Go  |
|------|-----------|-----|
| MQTT | Yes (TLS) | Yes |
| OSC  | No        |     |

# Endpoints

| Name    | Go   | TS   | Swift |  
|---------|------|------|-------|  
| Params  | Done | Done | Done  |
| Update  | Done | Done | Done  | 
| FS      | Done | Done | Done  |
| Relay   |      | Done | Done  |
| Metrics | Done | Done | Done  |
| Auth    |      | Done |       |
| Debug   | Done |      |       |

# TODO

- Decouple Swift Messaging
- Align Swift Managed Device
- Make channels reference counted?
