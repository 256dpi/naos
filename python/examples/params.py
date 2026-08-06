import asyncio
import sys

from naos import Session, collect_params, get_param, list_params
from naos.serial import SerialDevice, serial_find


async def main():
    # find device
    port = sys.argv[1] if len(sys.argv) > 1 else serial_find()
    if not port:
        print("no serial ports found")
        return
    print("using:", port)

    # open channel
    device = SerialDevice(port)
    channel = await device.open()

    try:
        # open session
        session = await Session.open(channel)

        # list parameters
        params = await list_params(session)
        for param in params:
            print(f"{param.ref}: {param.name} ({param.type.name}, {param.mode!r})")

        # collect all values
        updates = await collect_params(session, [p.ref for p in params], 0)
        values = {u.ref: u.value for u in updates}
        for param in params:
            print(f"{param.name} = {values.get(param.ref)!r}")

        # get device name
        print("device-name:", await get_param(session, "device-name"))

        # end session
        await session.end()
    finally:
        await channel.close()


if __name__ == "__main__":
    asyncio.run(main())
