import asyncio
import sys

from naos import Session
from naos.ble import BLEDevice, ble_scan


async def main():
    # scan for devices
    devices = await ble_scan()
    if not devices:
        print("no devices found")
        return
    print("devices:", devices)

    # select device
    if len(sys.argv) > 1:
        found = [d for d in devices if d.address == sys.argv[1] or d.name == sys.argv[1]]
        if not found:
            print("device not found")
            return
        descriptor = found[0]
    else:
        descriptor = devices[0]
    print("using:", descriptor)

    # open channel
    device = BLEDevice.from_descriptor(descriptor)
    channel = await device.open()

    try:
        # open session
        session = await Session.open(channel)
        print("session:", session.id())

        # ping device
        await session.ping()
        print("ping: ok")

        # get status and MTU
        print("status:", await session.status())
        print("mtu:", await session.get_mtu())

        # end session
        await session.end()
    finally:
        await channel.close()


if __name__ == "__main__":
    asyncio.run(main())
