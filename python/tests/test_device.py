import asyncio

import pytest

from naos import Channel, Message, Queue, read

from fake import FakeDeviceTransport


def test_message_build():
    msg = Message(0x1234, 0x42, b"\x01\x02")
    assert msg.build() == b"\x01\x34\x12\x42\x01\x02"
    assert msg.size() == 2

    msg = Message(1, 2, None)
    assert msg.build() == b"\x01\x01\x00\x02"
    assert msg.size() == 0


def test_message_parse():
    msg = Message.parse(b"\x01\x34\x12\x42\x01\x02")
    assert msg.session == 0x1234
    assert msg.endpoint == 0x42
    assert msg.data == b"\x01\x02"

    msg = Message.parse(b"\x01\x01\x00\x02")
    assert msg.session == 1
    assert msg.endpoint == 2
    assert msg.data is None

    assert Message.parse(b"\x02\x01\x00\x02") is None
    assert Message.parse(b"\x01\x01") is None


async def test_queue_timeout():
    queue = Queue()
    with pytest.raises(TimeoutError):
        await read(queue, 0.01)

    queue.push(Message(1, 2, None))
    msg = await read(queue, 0.01)
    assert msg.session == 1


async def test_channel_broadcast():
    transport = FakeDeviceTransport()
    channel = Channel(transport, None, 1)

    q1, q2 = Queue(), Queue()
    channel.subscribe(q1)
    channel.subscribe(q2)

    # session-less messages are broadcast to all queues
    transport.on_data(Message(0, 0x10, None))
    assert (await read(q1, 0.1)).endpoint == 0x10
    assert (await read(q2, 0.1)).endpoint == 0x10

    # unknown session messages are dropped
    transport.on_data(Message(42, 0x10, None))
    with pytest.raises(TimeoutError):
        await read(q1, 0.05)

    await channel.close()
    assert transport.closed


async def test_channel_session_routing():
    transport = FakeDeviceTransport()
    channel = Channel(transport, None, 1)

    q1, q2 = Queue(), Queue()
    channel.subscribe(q1)
    channel.subscribe(q2)

    # open session from q1
    await channel.write(q1, Message(0, 0, b"handle1"))
    await asyncio.sleep(0)

    # reply is routed to q1 only
    msg = await read(q1, 0.1)
    assert msg.endpoint == 0
    assert msg.data == b"handle1"
    sid = msg.session
    with pytest.raises(TimeoutError):
        await read(q2, 0.05)

    # session messages are routed to owner
    transport.on_data(Message(sid, 0x10, None))
    assert (await read(q1, 0.1)).endpoint == 0x10

    # other queues may not write to the session
    with pytest.raises(RuntimeError, match="wrong owner"):
        await channel.write(q2, Message(sid, 0x10, None))

    await channel.close()
