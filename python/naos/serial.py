from __future__ import annotations

import asyncio
import base64
import binascii
import os
from typing import Callable, List, Optional

from serial.tools import list_ports
from serial_asyncio import open_serial_connection

from .device import Channel, Device, Message, Transport

_known_prefixes = ["cu.SLAB", "cu.usbserial", "cu.usbmodem", "ttyUSB"]

# limit for garbage received without a newline
_max_buffer = 1 << 20


def serial_list() -> List[str]:
    """Return a list of all known serial ports."""

    # get paths, sort reverse to list combined ports with serial port first
    paths = sorted((port.device for port in list_ports.comports()), reverse=True)

    # filter to known prefixes
    return [
        path
        for path in paths
        if any(prefix in path for prefix in _known_prefixes)
    ]


def serial_find() -> Optional[str]:
    """Return the first best available serial port."""

    # list ports
    ports = serial_list()
    if not ports:
        return None

    # prefer USB modem ports
    for port in ports:
        if "usbmodem" in port:
            return port

    return ports[0]


class SerialDevice(Device):
    def __init__(self, path: str, baud_rate: int = 115200):
        # store path and baud rate
        self._path = path
        self._baud_rate = baud_rate
        self._ch: Optional[Channel] = None

    def id(self) -> str:
        return f"serial/{os.path.basename(self._path)}"

    def type(self) -> str:
        return "Serial"

    def name(self) -> str:
        return os.path.basename(self._path)

    async def open(self) -> Channel:
        # check channel
        if self._ch:
            raise RuntimeError("channel already open")

        # open port
        reader, writer = await open_serial_connection(
            url=self._path, baudrate=self._baud_rate
        )

        # create transport
        transport = SerialTransport(reader, writer)

        # create channel
        self._ch = Channel(transport, self, 1, self._clear)
        return self._ch

    def _clear(self):
        self._ch = None


class SerialTransport(Transport):
    def __init__(self, reader: asyncio.StreamReader, writer: asyncio.StreamWriter):
        self._reader = reader
        self._writer = writer
        self._lock = asyncio.Lock()
        self._task: Optional[asyncio.Task] = None

    def start(self, on_data: Callable[[Message], None], on_close: Callable[[], None]):
        self._task = asyncio.get_running_loop().create_task(
            self._read(on_data, on_close)
        )

    async def _read(
        self, on_data: Callable[[Message], None], on_close: Callable[[], None]
    ):
        try:
            buffer = b""

            while True:
                # read data
                chunk = await self._reader.read(4096)
                if not chunk:
                    break

                # add chunk to buffer
                buffer += chunk

                # split buffer into lines, keep last incomplete line
                *lines, buffer = buffer.split(b"\n")

                # drop buffer if no newline is found within limit
                if len(buffer) > _max_buffer:
                    buffer = b""

                # process all complete lines
                for line in lines:
                    line = line.rstrip(b"\r")
                    if line.startswith(b"NAOS!"):
                        try:
                            frame = base64.b64decode(line[5:])
                        except (binascii.Error, ValueError):
                            continue
                        msg = Message.parse(frame)
                        if msg:
                            on_data(msg)
        except asyncio.CancelledError:
            raise
        except Exception:
            pass  # device disconnected
        finally:
            on_close()

    async def write(self, msg: Message):
        async with self._lock:
            frame = b"\nNAOS!" + base64.b64encode(msg.build()) + b"\n"
            self._writer.write(frame)
            await self._writer.drain()

    async def close(self):
        # stop reader
        if self._task and not self._task.done():
            self._task.cancel()

        # close writer
        try:
            self._writer.close()
            await asyncio.wait_for(self._writer.wait_closed(), 1.0)
        except Exception:
            pass
