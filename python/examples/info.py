import asyncio
import sys

from naos import (
    MetricType,
    Session,
    describe_metric,
    get_time,
    get_time_info,
    list_dir,
    list_metrics,
    read_double_metrics,
    read_float_metrics,
    read_long_metrics,
)
from naos.serial import SerialDevice, serial_find

readers = {
    MetricType.LONG: read_long_metrics,
    MetricType.FLOAT: read_float_metrics,
    MetricType.DOUBLE: read_double_metrics,
}


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

        # get time and offset
        print("time:  ", await get_time(session))
        print("offset:", await get_time_info(session))

        # list metrics with layouts and values
        for metric in await list_metrics(session):
            layout = await describe_metric(session, metric.ref)
            values = await readers[metric.type](session, metric.ref)
            print(
                f"metric {metric.ref}: {metric.name} "
                f"({metric.kind.name}/{metric.type.name}) "
                f"keys={layout.keys} values={values}"
            )

        # list root directory
        for entry in await list_dir(session, "/"):
            kind = "dir " if entry.is_dir else "file"
            print(f"{kind} /{entry.name} ({entry.size} bytes)")

        # end session
        await session.end()
    finally:
        await channel.close()


if __name__ == "__main__":
    asyncio.run(main())
