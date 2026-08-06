from __future__ import annotations

import struct
from datetime import datetime, timedelta, timezone

from .session import Session
from .utils import pack

_time_endpoint = 0x09


async def get_time(s: Session, timeout: float = 5.0) -> datetime:
    """Return the device's current wall-clock time in UTC at millisecond
    resolution."""

    # send command
    await s.send(_time_endpoint, pack("o", 0), 0)

    # receive reply
    reply, _ = await s.receive(_time_endpoint, False, timeout)

    # verify reply
    if len(reply) != 8:
        raise RuntimeError("invalid reply")

    # parse epoch milliseconds
    ms = struct.unpack("<q", reply)[0]

    return datetime.fromtimestamp(ms / 1000, tz=timezone.utc)


async def set_time(s: Session, time: datetime, timeout: float = 5.0):
    """Set the device's wall-clock time in UTC at millisecond resolution."""

    # build command
    cmd = bytes([1]) + struct.pack("<q", round(time.timestamp() * 1000))

    # send command
    await s.send(_time_endpoint, cmd, timeout)


async def get_time_info(s: Session, timeout: float = 5.0) -> timedelta:
    """Return the device's current timezone offset from UTC."""

    # send command
    await s.send(_time_endpoint, pack("o", 2), 0)

    # receive reply
    reply, _ = await s.receive(_time_endpoint, False, timeout)

    # verify reply
    if len(reply) != 4:
        raise RuntimeError("invalid reply")

    # parse offset in seconds (signed)
    return timedelta(seconds=struct.unpack("<i", reply)[0])
