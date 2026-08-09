# naos

Python driver for NAOS devices.

## Status

Supported transports: Serial, BLE. Supported endpoints: Params, FS, Metrics,
Time. See `PARITY.md` in the repository root for the full parity matrix.

## Installation

Install directly from the repository:

```
pip install "git+https://github.com/256dpi/naos.git#subdirectory=python"
```

Or install from a local checkout:

```
pip install ./python
```

BLE support requires the optional `bleak` dependency:

```
pip install "./python[ble]"
```

## Usage

The driver is asyncio-based. Timeouts are given in seconds.

```python
import asyncio

from naos import Session
from naos.serial import SerialDevice, serial_find

async def main():
    device = SerialDevice(serial_find())
    channel = await device.open()
    session = await Session.open(channel)
    await session.ping()
    await session.end()
    await channel.close()

asyncio.run(main())
```

To connect over BLE, scan for devices and use a `BLEDevice` instead:

```python
from naos.ble import BLEDevice, ble_find

descriptor = await ble_find()
device = BLEDevice.from_descriptor(descriptor)
channel = await device.open()
```

See `examples/ping.py` and `examples/ble.py` for runnable examples.

## Tests

```
pip install -e "./python[test]"
pytest python/tests
```
