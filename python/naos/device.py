from __future__ import annotations

import asyncio
import struct
from abc import ABC, abstractmethod
from typing import Callable, Optional


class Message:
    """Message represents a message exchanged between a device and client."""

    __slots__ = ("session", "endpoint", "data")

    def __init__(self, session: int, endpoint: int, data: Optional[bytes]):
        self.session = session  # uint16
        self.endpoint = endpoint  # uint8
        self.data = data

    def size(self) -> int:
        """Returns the size of the message."""

        return len(self.data) if self.data else 0

    @classmethod
    def parse(cls, data: bytes) -> Optional[Message]:
        if len(data) < 4 or data[0] != 1:
            return None

        session, endpoint = struct.unpack_from("<HB", data, 1)

        return cls(session, endpoint, data[4:] if len(data) > 4 else None)

    def build(self) -> bytes:
        return struct.pack("<BHB", 1, self.session, self.endpoint) + (self.data or b"")

    def __repr__(self):
        return f"Message(session={self.session}, endpoint={self.endpoint:#x}, size={self.size()})"


class Queue:
    """Queue is used to receive messages from a channel."""

    def __init__(self):
        self._queue: asyncio.Queue[Message] = asyncio.Queue()

    def push(self, msg: Message):
        self._queue.put_nowait(msg)

    async def pop(self, timeout: float) -> Optional[Message]:
        if timeout <= 0:
            return await self._queue.get()
        try:
            return await asyncio.wait_for(self._queue.get(), timeout)
        except asyncio.TimeoutError:
            return None


class Transport(ABC):
    """Transport exchanges raw messages with a device or peer."""

    @abstractmethod
    def start(self, on_data: Callable[[Message], None], on_close: Callable[[], None]):
        ...

    @abstractmethod
    async def write(self, msg: Message):
        ...

    @abstractmethod
    async def close(self):
        ...


class Device(ABC):
    """Device represents a device that can be communicated with."""

    @abstractmethod
    def id(self) -> str:
        """Returns a stable identifier for the device."""

    @abstractmethod
    def type(self) -> str:
        """Returns the device transport type."""

    @abstractmethod
    def name(self) -> str:
        """Returns a user-facing device name."""

    @abstractmethod
    async def open(self) -> Channel:
        """Open opens a channel to the device. An opened channel must fail or
        be closed before another channel can be opened."""


class Channel:
    """Channel wraps a raw transport in a session-aware channel."""

    def __init__(
        self,
        transport: Transport,
        device: Optional[Device],
        width: int,
        on_close: Optional[Callable[[], None]] = None,
    ):
        self._tr = transport
        self._dev = device
        self._width = width
        self._on_close = on_close
        self._closed = False
        self._done = asyncio.Event()
        self._queues: set[Queue] = set()
        self._opening: dict[str, Queue] = {}
        self._sessions: dict[int, Queue] = {}
        self._closing: dict[int, Queue] = {}
        self._tasks: set[asyncio.Task] = set()
        self._tr.start(self._handle_data, self._handle_close)

    def width(self) -> int:
        return self._width

    def device(self) -> Optional[Device]:
        return self._dev

    def subscribe(self, queue: Queue):
        self._queues.add(queue)

    def unsubscribe(self, queue: Queue):
        self._queues.discard(queue)

        for handle, owner in list(self._opening.items()):
            if owner is queue:
                del self._opening[handle]
        for session, owner in list(self._sessions.items()):
            if owner is queue:
                del self._sessions[session]
                self._closing.pop(session, None)
        for session, owner in list(self._closing.items()):
            if owner is queue:
                del self._closing[session]

    async def write(self, queue: Optional[Queue], msg: Message):
        if not queue:
            await self._tr.write(msg)
            return

        if msg.session != 0:
            owner = self._sessions.get(msg.session)
            if owner and owner is not queue:
                raise RuntimeError("wrong owner")

        if msg.session == 0 and msg.endpoint == 0x0:
            self._opening[_handle_key(msg)] = queue
        if msg.session != 0 and msg.endpoint == 0xFF:
            self._closing[msg.session] = queue

        try:
            await self._tr.write(msg)
        except Exception:
            if (
                msg.session == 0
                and msg.endpoint == 0x0
                and self._opening.get(_handle_key(msg)) is queue
            ):
                del self._opening[_handle_key(msg)]
            if (
                msg.session != 0
                and msg.endpoint == 0xFF
                and self._closing.get(msg.session) is queue
            ):
                del self._closing[msg.session]

            await self.close()

            raise

    async def close(self):
        if self._closed:
            return

        self._closed = True
        try:
            await self._tr.close()
        finally:
            if self._on_close:
                self._on_close()
            self._done.set()

    async def wait_closed(self):
        """Wait until the channel is closed."""

        await self._done.wait()

    def _handle_data(self, msg: Message):
        for queue in self._route(msg):
            queue.push(msg)

    def _handle_close(self):
        if not self._closed:
            task = asyncio.get_running_loop().create_task(self.close())
            self._tasks.add(task)
            task.add_done_callback(self._tasks.discard)

    def _route(self, msg: Message) -> list[Queue]:
        if msg.endpoint == 0x0:
            owner = self._opening.get(_handle_key(msg))
            if owner and owner in self._queues:
                del self._opening[_handle_key(msg)]
                self._sessions[msg.session] = owner
                return [owner]

        if msg.session != 0:
            owner = self._sessions.get(msg.session)
            if owner and owner in self._queues:
                if msg.endpoint == 0xFF and msg.size() == 0:
                    del self._sessions[msg.session]
                    self._closing.pop(msg.session, None)
                return [owner]

            self._sessions.pop(msg.session, None)
            self._closing.pop(msg.session, None)
            return []

        return list(self._queues)


async def read(queue: Queue, timeout: float) -> Message:
    """Read reads a message from the queue."""

    msg = await queue.pop(timeout)
    if not msg:
        raise TimeoutError("timeout")

    return msg


def _handle_key(msg: Message) -> str:
    return msg.data.decode() if msg.data else ""
