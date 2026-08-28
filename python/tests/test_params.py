import pytest

from naos import (
    Channel,
    ParamMode,
    ParamType,
    Session,
    clear_param,
    collect_params,
    get_param,
    list_params,
    read_param,
    set_param,
    write_param,
)

from fake import FakeDeviceTransport


async def test_params_get_set():
    transport = FakeDeviceTransport()
    channel = Channel(transport, None, 1)
    session = await Session.open(channel)

    assert await get_param(session, "app-name") == b"test"

    await set_param(session, "app-name", b"other")
    assert await get_param(session, "app-name") == b"other"

    await session.end()
    await channel.close()


async def test_params_list():
    transport = FakeDeviceTransport()
    channel = Channel(transport, None, 1)
    session = await Session.open(channel)

    params = await list_params(session)
    assert len(params) == 2
    assert params[0].ref == 1
    assert params[0].type == ParamType.STRING
    assert params[0].mode == ParamMode.SYSTEM
    assert params[0].name == "app-name"
    assert params[1].ref == 2
    assert params[1].type == ParamType.LONG
    assert params[1].mode == ParamMode.APPLICATION
    assert params[1].name == "counter"

    await session.end()
    await channel.close()


async def test_params_read_write_clear():
    transport = FakeDeviceTransport()
    channel = Channel(transport, None, 1)
    session = await Session.open(channel)

    assert await read_param(session, 2) == b"42"

    await write_param(session, 2, b"43")
    assert await read_param(session, 2) == b"43"

    await clear_param(session, 2)
    assert await read_param(session, 2) == b""

    await session.end()
    await channel.close()


async def test_params_collect():
    transport = FakeDeviceTransport()
    channel = Channel(transport, None, 1)
    session = await Session.open(channel)

    # collect all
    updates = await collect_params(session, [], 0)
    assert len(updates) == 2
    assert updates[0].ref == 1
    assert updates[0].age == 10
    assert updates[0].value == b"test"

    # collect selected refs
    updates = await collect_params(session, [2], 0)
    assert len(updates) == 1
    assert updates[0].ref == 2

    # collect since age
    updates = await collect_params(session, [], 15)
    assert len(updates) == 1
    assert updates[0].ref == 2

    # reject out-of-range refs
    with pytest.raises(ValueError, match="exceeds bitmap capacity"):
        await collect_params(session, [256], 0)

    await session.end()
    await channel.close()
