import asyncio
import sys

from naos import Session
from naos.serial import SerialDevice, serial_find, serial_list


async def main():
    # find device
    port = sys.argv[1] if len(sys.argv) > 1 else serial_find()
    if not port:
        print("no serial ports found")
        return
    print("ports:", serial_list())
    print("using:", port)

    # open channel
    device = SerialDevice(port)
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
