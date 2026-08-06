import asyncio
import base64

from naos import Message
from naos.serial import SerialTransport


class StubWriter:
    def __init__(self):
        self.data = b""
        self.closed = False

    def write(self, data):
        self.data += data

    async def drain(self):
        pass

    def close(self):
        self.closed = True

    async def wait_closed(self):
        pass


async def test_serial_read_framing():
    reader = asyncio.StreamReader()
    writer = StubWriter()
    transport = SerialTransport(reader, writer)

    received = []
    closed = asyncio.Event()
    transport.start(received.append, closed.set)

    # feed debug noise, a valid frame, garbage, and a partial line
    frame = Message(1, 2, b"\x03").build()
    reader.feed_data(b"some debug log\r\n")
    reader.feed_data(b"NAOS!" + base64.b64encode(frame) + b"\r\n")
    reader.feed_data(b"NAOS!not-base64!\n")
    reader.feed_data(b"NAOS!")  # incomplete line
    reader.feed_eof()

    await asyncio.wait_for(closed.wait(), 1)
    assert len(received) == 1
    assert received[0].session == 1
    assert received[0].endpoint == 2
    assert received[0].data == b"\x03"


async def test_serial_write_framing():
    reader = asyncio.StreamReader()
    writer = StubWriter()
    transport = SerialTransport(reader, writer)
    transport.start(lambda msg: None, lambda: None)

    msg = Message(1, 2, b"\x03")
    await transport.write(msg)
    assert writer.data == b"\nNAOS!" + base64.b64encode(msg.build()) + b"\n"

    await transport.close()
    assert writer.closed
