---
title: Command Line Utility
---

# Command Line Utility

The following commands are offered by the `naos` command line utility:

```
Networked Artifacts Operating System
© Joël Gähwiler
https://github.com/256dpi/naos

Project Management:
  create   Create a new naos project in the current directory.
  install  Download required dependencies to the 'naos' subdirectory.
  build    Build all source files.
  flash    Flash the previously built binary to an attached device.
  attach   Open a serial communication with an attached device.
  run      Run 'build', 'flash' and 'attach' sequentially.
  format   Format all source files in the 'src' subdirectory.

Fleet Management:
  list     List all devices listed in the inventory.
  collect  Collect devices and add them to the inventory.
  ping     Ping devices.
  send     Send a message to devices.
  discover Discover all parameters of a device.
  get      Read a parameter from devices.
  set      Set a parameter on devices.
  unset    Unset a parameter on devices.
  monitor  Monitor heartbeats from devices.
  record   Record log messages from devices.
  debug    Gather debug information from devices.
  update   Send the previously built binary to devices.

Usage:
  naos create [--cmake --force]
  naos install [--force]
  naos build [--clean --app-only]
  naos flash [<device>] [--erase --app-only]
  naos attach [<device>] [--simple]
  naos run [<device>] [--clean --app-only --erase --simple]
  naos format
  naos list
  naos collect [--clear --duration=<time>]
  naos ping [<pattern>] [--timeout=<time>]
  naos send <topic> [--] <message> [<pattern>] [--timeout=<time>]
  naos discover [<pattern>] [--timeout=<time>]
  naos get <param> [<pattern>] [--timeout=<time>]
  naos set <param> [--] <value> [<pattern>] [--timeout=<time>]
  naos unset <param> [<pattern>] [--timeout=<time>]
  naos monitor [<pattern>] [--timeout=<time>]
  naos record [<pattern>] [--timeout=<time>]
  naos debug [<pattern>] [--delete --duration=<time>]
  naos update [<pattern>] [--jobs=<count> --timeout=<time>]
  naos help

Options:
  --cmake               Create required CMake files for IDEs like CLion.
  --force               Reinstall dependencies when they already exist.
  --clean               Clean all build artifacts before building again.
  --erase               Erase completely before flashing new image.
  --app-only            Only build or flash the application.
  --simple              Use simple serial tool.
  --clear               Remove not available devices from inventory.
  --delete              Delete loaded coredumps from the devices.
  -d --duration=<time>  Operation duration [default: 2s].
  -t --timeout=<time>   Operation timeout [default: 5s].
  -j --jobs=<count>     Number of simultaneous update jobs [default: 10].
```
