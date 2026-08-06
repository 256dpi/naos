from datetime import datetime, timedelta, timezone

from naos import Channel, Session, get_time, get_time_info, set_time

from fake import FakeDeviceTransport


async def test_time():
    transport = FakeDeviceTransport()
    channel = Channel(transport, None, 1)
    session = await Session.open(channel)

    # get time
    time = await get_time(session)
    assert time == datetime.fromtimestamp(1700000000, tz=timezone.utc)

    # set time
    new = datetime(2026, 8, 6, 12, 30, 0, 500000, tzinfo=timezone.utc)
    await set_time(session, new)
    assert transport.time_ms == round(new.timestamp() * 1000)
    assert await get_time(session) == new

    # get info
    assert await get_time_info(session) == timedelta(hours=1)

    await channel.close()
