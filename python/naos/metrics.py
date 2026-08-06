from __future__ import annotations

import struct
from dataclasses import dataclass
from enum import IntEnum
from typing import List

from .session import Session
from .utils import pack

_metrics_endpoint = 0x05


class MetricKind(IntEnum):
    COUNTER = 0
    GAUGE = 1


class MetricType(IntEnum):
    LONG = 0
    FLOAT = 1
    DOUBLE = 2


@dataclass
class MetricInfo:
    ref: int
    kind: MetricKind
    type: MetricType
    name: str
    size: int


@dataclass
class MetricLayout:
    keys: List[str]
    values: List[List[str]]


async def list_metrics(s: Session, timeout: float = 5.0) -> List[MetricInfo]:
    """Return a list of all metrics."""

    # send command
    await s.send(_metrics_endpoint, pack("o", 0), 0)

    # prepare list
    result = []

    while True:
        # receive reply or return list on ack
        reply, ack = await s.receive(_metrics_endpoint, True, timeout)
        if ack:
            break

        # verify reply
        if len(reply) < 4:
            raise RuntimeError("invalid reply")

        # parse reply
        ref, kind, type_, size = reply[0], reply[1], reply[2], reply[3]
        name = reply[4:].decode()

        # append info
        result.append(MetricInfo(ref, MetricKind(kind), MetricType(type_), name, size))

    return result


async def describe_metric(s: Session, ref: int, timeout: float = 5.0) -> MetricLayout:
    """Return the layout of the referenced metric."""

    # send command
    await s.send(_metrics_endpoint, pack("oo", 1, ref), 0)

    # prepare lists
    keys = {}
    values = {}

    while True:
        # receive reply
        reply, ack = await s.receive(_metrics_endpoint, True, timeout)
        if ack:
            break

        # verify reply
        if len(reply) < 1:
            raise RuntimeError("invalid reply")

        # handle key
        if reply[0] == 0:
            # verify reply
            if len(reply) < 3:
                raise RuntimeError("invalid reply")

            # add key
            keys[reply[1]] = reply[2:].decode()
            values[reply[1]] = {}

            continue

        # handle value
        if reply[0] == 1:
            # verify reply
            if len(reply) < 4:
                raise RuntimeError("invalid reply")

            # check key index
            if reply[1] not in values:
                raise RuntimeError(f"invalid key index: {reply[1]}")

            # add value
            values[reply[1]][reply[2]] = reply[3:].decode()

            continue

        raise RuntimeError("invalid reply")

    return MetricLayout(
        keys=[keys[num] for num in sorted(keys)],
        values=[
            [values[num][sub] for sub in sorted(values[num])] for num in sorted(keys)
        ],
    )


async def read_metrics(s: Session, ref: int, timeout: float = 5.0) -> bytes:
    """Return the raw values of the referenced metric."""

    # send command
    await s.send(_metrics_endpoint, pack("oo", 2, ref), 0)

    # receive reply
    reply, _ = await s.receive(_metrics_endpoint, False, timeout)

    return reply or b""


async def read_long_metrics(s: Session, ref: int, timeout: float = 5.0) -> List[int]:
    """Return the values of the referenced long metric."""

    return _convert(await read_metrics(s, ref, timeout), "i", 4)


async def read_float_metrics(s: Session, ref: int, timeout: float = 5.0) -> List[float]:
    """Return the values of the referenced float metric."""

    return _convert(await read_metrics(s, ref, timeout), "f", 4)


async def read_double_metrics(
    s: Session, ref: int, timeout: float = 5.0
) -> List[float]:
    """Return the values of the referenced double metric."""

    return _convert(await read_metrics(s, ref, timeout), "d", 8)


def _convert(data: bytes, code: str, width: int) -> list:
    # verify length
    if len(data) % width != 0:
        raise RuntimeError(f"invalid metric payload length: {len(data)}")

    return list(struct.unpack(f"<{len(data) // width}{code}", data))
