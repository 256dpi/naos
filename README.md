# naos

**The [Networked Artifacts Operating System](https://github.com/shiftr-io/naos).**

This repository contains the naos go library and command line utility.

## CLI

The following commands are offered by the `naos` command line utility.

```
Project Management:
  create   Will create a new naos project in the current directory.
  install  Will download required dependencies to the 'naos' subdirectory.
  build    Will build all source files.
  flash    Will flash the previously built binary to an attached device.
  attach   Will open a serial communication with an attached device.
  run      Will run 'build', 'flash' and 'attach' sequentially.
  format   Will format all source files in the 'src' subdirectory.

Configuration Management:
  scan     Will scan for bluetooth devices and print their configuration.
  rename   Will reset the device name of the device with the specified address.

Fleet Management:
  list     Will list all devices listed in the inventory.
  collect  Will collect devices and add them to the inventory.
  get      Will read a parameter value from devices.
  set      Will set a parameter value on devices.
  unset    Will unset a parameter on devices.
  monitor  Will monitor heartbeats from devices.
  record   Will record log messages from devices.
  debug    Will gather debug information from devices.
  update   Will send the previously built binary to devices.

Usage:
  naos create [--cmake --force]
  naos install [--force]
  naos build [--clean --app-only]
  naos flash [<device>] [--erase --app-only]
  naos attach [<device>] [--simple]
  naos run [<device>] [--clean --app-only --erase --simple]
  naos format
  naos scan [--duration=<time>]
  naos rename <address> <name>
  naos list
  naos collect [--clear --duration=<time>]
  naos get <param> [<pattern>] [--timeout=<time>]
  naos set <param> <value> [<pattern>] [--timeout=<time>]
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
