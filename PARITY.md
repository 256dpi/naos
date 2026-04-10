# Parity

This document tracks the parity of features among the Go, TS, and Swift NAOS drivers, as well as the availability of features in the CLI tools and Swift desktop application.

## Channels

| Name    | Securable  | Go  | TS  | Swift | 
|---------|------------|-----|-----|-------|
| BLE     | Yes        | Yes | Yes | Yes   |
| HTTP    | Yes (TLS)  | Yes | Yes | Yes   |
| MQTT    | Yes (TLS)  | Yes |     |       |
| Serial  | No (Wired) | Yes | Yes | Yes   |
| Connect | Yes (TLS)  | Yes |     |       |

## Endpoints

| Name    | Go   | TS   | Swift | Explorer | Desktop | Fleet |  
|---------|------|------|-------|----------|---------|-------|  
| Params  | Done | Done | Done  | Started  | Done    | Done  |
| Update  | Done | Done | Done  | Started  | Done    | Done  | 
| FS      | Done | Done | Done  | Started  | Done    |       |
| Relay   |      | Done | Done  |          | Done    |       |
| Metrics | Done | Done | Done  | Started  | Done    |       |
| Auth    |      | Done |       |          |         |       |
| Debug   | Done | Done |       | Started  |         | Done  |

## Notes and Todos

- Use Go's Channel design in TS and Swift
- Decouple Swift Messaging
- Align Swift Managed Device
