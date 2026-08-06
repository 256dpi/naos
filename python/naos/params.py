from __future__ import annotations

from dataclasses import dataclass
from enum import IntEnum, IntFlag
from typing import List, Sequence

from .session import Session
from .utils import pack, unpack

_params_endpoint = 0x01


class ParamType(IntEnum):
    RAW = 0
    STRING = 1
    BOOL = 2
    LONG = 3
    DOUBLE = 4
    ACTION = 5


class ParamMode(IntFlag):
    VOLATILE = 1 << 0
    SYSTEM = 1 << 1
    APPLICATION = 1 << 2
    LOCKED = 1 << 4


def _valid_param_type(type_: int) -> bool:
    return type_ in (
        ParamType.RAW,
        ParamType.STRING,
        ParamType.BOOL,
        ParamType.LONG,
        ParamType.DOUBLE,
        ParamType.ACTION,
    )


def _valid_param_mode(mode: int) -> bool:
    mask = (
        ParamMode.VOLATILE | ParamMode.SYSTEM | ParamMode.APPLICATION | ParamMode.LOCKED
    )
    return mode & ~mask == 0


@dataclass
class ParamInfo:
    ref: int
    type: ParamType
    mode: ParamMode
    name: str


@dataclass
class ParamUpdate:
    ref: int
    age: int
    value: bytes


async def get_param(s: Session, name: str, timeout: float = 5.0) -> bytes:
    """Return the value of the named parameter."""

    # send command
    cmd = pack("os", 0, name)
    await s.send(_params_endpoint, cmd, 0)

    # receive value
    data, _ = await s.receive(_params_endpoint, False, timeout)

    return data or b""


async def set_param(s: Session, name: str, value: bytes, timeout: float = 5.0):
    """Set the value of the named parameter."""

    # send command
    cmd = pack("osob", 1, name, 0, value)
    await s.send(_params_endpoint, cmd, timeout)


async def list_params(s: Session, timeout: float = 5.0) -> List[ParamInfo]:
    """Return a list of all parameters."""

    # send command
    await s.send(_params_endpoint, pack("o", 2), 0)

    # prepare list
    result = []

    while True:
        # receive reply or return list on ack
        reply, ack = await s.receive(_params_endpoint, True, timeout)
        if ack:
            break

        # verify reply
        if len(reply) < 4:
            raise RuntimeError("invalid reply")

        # parse reply
        ref, type_, mode = reply[0], reply[1], reply[2]
        name = reply[3:].decode()

        # check type and mode
        if not _valid_param_type(type_) or not _valid_param_mode(mode):
            raise RuntimeError("invalid type or mode")

        # append info
        result.append(ParamInfo(ref, ParamType(type_), ParamMode(mode), name))

    return result


async def read_param(s: Session, ref: int, timeout: float = 5.0) -> bytes:
    """Return the value of the referenced parameter."""

    # send command
    await s.send(_params_endpoint, pack("oo", 3, ref), 0)

    # receive value
    data, _ = await s.receive(_params_endpoint, False, timeout)

    return data or b""


async def write_param(s: Session, ref: int, value: bytes, timeout: float = 5.0):
    """Set the value of the referenced parameter."""

    # send command
    cmd = pack("oob", 4, ref, value)
    await s.send(_params_endpoint, cmd, timeout)


async def collect_params(
    s: Session, refs: Sequence[int], since: int, timeout: float = 5.0
) -> List[ParamUpdate]:
    """Return a list of parameter updates."""

    # prepare map
    map_ = (1 << 64) - 1
    if refs:
        map_ = 0
        for ref in refs:
            if ref >= 64:
                raise ValueError(f"ref {ref} exceeds bitmap capacity")
            map_ |= 1 << ref

    # send command
    cmd = pack("oqq", 5, map_, since)
    await s.send(_params_endpoint, cmd, 0)

    # prepare list
    result = []

    while True:
        # receive reply or return list on ack
        reply, ack = await s.receive(_params_endpoint, True, timeout)
        if ack:
            break

        # verify reply
        if len(reply) < 9:
            raise RuntimeError("invalid reply")

        # parse reply
        ref, age, value = unpack("oqb", reply)

        # append info
        result.append(ParamUpdate(ref, age, value))

    return result


async def clear_param(s: Session, ref: int, timeout: float = 5.0):
    """Clear the value of the referenced parameter."""

    # send command
    await s.send(_params_endpoint, pack("oo", 6, ref), timeout)
