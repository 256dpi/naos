import pytest

from naos import Channel, Session, Status

from fake import FakeDeviceTransport


async def test_session_lifecycle():
    transport = FakeDeviceTransport()
    channel = Channel(transport, None, 1)

    session = await Session.open(channel)
    assert session.id() == 1
    assert session.channel() is channel

    await session.ping()

    mtu = await session.get_mtu()
    assert mtu == 120

    await session.end()
    await channel.close()


async def test_session_status_unlock():
    transport = FakeDeviceTransport(password="secret")
    channel = Channel(transport, None, 1)

    session = await Session.open(channel)

    status = await session.status()
    assert status == Status.LOCKED

    assert not await session.unlock("wrong")
    assert await session.unlock("secret")

    status = await session.status()
    assert status == Status(0)

    await session.end()
    await channel.close()


async def test_session_query_unknown():
    transport = FakeDeviceTransport()
    channel = Channel(transport, None, 1)

    session = await Session.open(channel)
    assert not await session.query(0x42)

    await session.end()
    await channel.close()


async def test_session_isolation():
    transport = FakeDeviceTransport()
    channel = Channel(transport, None, 1)

    s1 = await Session.open(channel)
    s2 = await Session.open(channel)
    assert s1.id() != s2.id()

    await s1.ping()
    await s2.ping()

    await s1.end()
    await s2.end()
    await channel.close()


async def test_session_open_timeout():
    transport = FakeDeviceTransport()
    transport.handle = lambda msg: []  # device never replies
    channel = Channel(transport, None, 1)

    with pytest.raises(TimeoutError):
        await Session.open(channel, timeout=0.05)

    await channel.close()
