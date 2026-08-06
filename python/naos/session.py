from __future__ import annotations

import time
from enum import IntFlag
from typing import Optional, Tuple

from .device import Channel, Message, Queue, read
from .utils import pack, random_handle, unpack


class Status(IntFlag):
    LOCKED = 1 << 0


class Session:
    """Session represents a message session with a device."""

    @classmethod
    async def open(cls, channel: Channel, timeout: float = 5.0) -> Session:
        # prepare queue
        queue = Queue()

        # subscribe to channel
        channel.subscribe(queue)

        ok = False
        try:
            # prepare handle
            handle = random_handle(16)

            # begin session
            await channel.write(queue, Message(0, 0, handle.encode()))

            # await reply
            sid = None
            deadline = time.monotonic() + timeout
            while True:
                remaining = deadline - time.monotonic()
                if remaining <= 0:
                    raise TimeoutError("timeout")
                reply = await read(queue, remaining)
                if reply.endpoint == 0 and (reply.data or b"").decode() == handle:
                    sid = reply.session
                    break

            ok = True
            return cls(sid, channel, queue)
        finally:
            if not ok:
                channel.unsubscribe(queue)

    def __init__(self, sid: int, channel: Channel, queue: Queue):
        self._sid = sid
        self._ch = channel
        self._qu = queue
        self._mtu = 0

    def id(self) -> int:
        return self._sid

    def channel(self) -> Channel:
        return self._ch

    async def ping(self, timeout: float = 5.0):
        # write command
        await self._write(Message(self._sid, 0xFE, None))

        # read reply
        msg = await self.read(timeout)

        # verify reply
        if msg.endpoint != 0xFE or msg.size() != 1:
            raise RuntimeError("invalid reply")
        elif msg.data[0] != 1:
            raise RuntimeError(f"session error: {msg.data[0]}")

    async def query(self, endpoint: int, timeout: float = 5.0) -> bool:
        # write command
        await self._write(Message(self._sid, endpoint, None))

        # read reply
        msg = await self.read(timeout)

        # verify reply
        if msg.endpoint != 0xFE or msg.size() != 1:
            raise RuntimeError("invalid reply")

        return msg.data[0] == 1

    async def receive(
        self, endpoint: int, expect_ack: bool, timeout: float = 5.0
    ) -> Tuple[Optional[bytes], bool]:
        # await message
        msg = await self.read(timeout)

        # handle ack
        if msg.endpoint == 0xFE:
            # check size
            if msg.size() != 1:
                raise RuntimeError(f"invalid ack size: {msg.size()}")

            # check if OK
            if msg.data[0] == 1:
                if expect_ack:
                    return None, True
                else:
                    raise RuntimeError("unexpected ack")

            raise _parse_error(msg.data[0])

        # check endpoint
        if msg.endpoint != endpoint:
            raise RuntimeError(f"unexpected endpoint: {msg.endpoint}")

        return msg.data, False

    async def send(self, endpoint: int, data: bytes, ack_timeout: float):
        # write message
        await self._write(Message(self._sid, endpoint, data))

        # return if timeout is zero
        if ack_timeout == 0:
            return

        # await reply
        msg = await self.read(ack_timeout)

        # check reply
        if msg.size() != 1 or msg.endpoint != 0xFE:
            raise RuntimeError("invalid reply")
        elif msg.data[0] != 1:
            raise _parse_error(msg.data[0])

    async def status(self, timeout: float = 5.0) -> Status:
        # write command
        cmd = pack("o", 0)
        await self.send(0xFD, cmd, 0)

        # await reply
        reply, _ = await self.receive(0xFD, False, timeout)

        # verify reply
        if len(reply) != 1:
            raise RuntimeError("invalid reply")

        # unpack status
        status = unpack("o", reply)[0]

        return Status(status)

    async def unlock(self, password: str, timeout: float = 5.0) -> bool:
        # prepare command
        cmd = pack("os", 1, password)
        await self.send(0xFD, cmd, 0)

        # await reply
        reply, _ = await self.receive(0xFD, False, timeout)

        # verify reply
        if len(reply) != 1:
            raise RuntimeError("invalid reply")

        return reply[0] == 1

    async def get_mtu(self, timeout: float = 5.0) -> int:
        # return cached value
        if self._mtu > 0:
            return self._mtu

        # write command
        cmd = pack("o", 2)
        await self.send(0xFD, cmd, 0)

        # await reply
        reply, _ = await self.receive(0xFD, False, timeout)

        # verify reply
        if len(reply) != 2:
            raise RuntimeError("invalid reply")

        # cache value
        self._mtu = unpack("h", reply)[0]

        return self._mtu

    async def end(self, timeout: float = 5.0):
        try:
            # write command
            await self._write(Message(self._sid, 0xFF, None))

            # return if timeout is zero
            if timeout == 0:
                return

            # read reply
            msg = await self.read(timeout)

            # verify reply
            if msg.endpoint != 0xFF or msg.size() > 0:
                raise RuntimeError("invalid reply")
        finally:
            # unsubscribe from channel
            self._ch.unsubscribe(self._qu)

    async def read(self, timeout: float) -> Message:
        msg = await read(self._qu, timeout)
        if msg.session != self._sid:
            raise RuntimeError("invalid message")
        return msg

    async def _write(self, msg: Message):
        await self._ch.write(self._qu, msg)


def _parse_error(num: int) -> Exception:
    if num == 2:
        return RuntimeError("invalid")
    elif num == 3:
        return RuntimeError("unknown")
    elif num == 4:
        return RuntimeError("error")
    elif num == 5:
        return RuntimeError("locked")
    else:
        return RuntimeError(f"unexpected reply: {num}")
