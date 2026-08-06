# naos

Python driver for NAOS devices.

## Status

Supported transports: Serial. Supported endpoints: Params.
See `PARITY.md` in the repository root for the full parity matrix.

## Installation

Install directly from the repository:

```
pip install "git+https://github.com/256dpi/naos.git#subdirectory=python"
```

To install a specific branch, tag, or commit, add a `@<ref>` suffix:

```
pip install "git+https://github.com/256dpi/naos.git@v0.15.0#subdirectory=python"
```

Or install from a local checkout:

```
pip install ./python
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

See `examples/ping.py` for a runnable example.

## Tests

```
pip install -e "./python[test]"
pytest python/tests
```
