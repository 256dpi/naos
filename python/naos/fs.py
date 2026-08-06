from __future__ import annotations

from dataclasses import dataclass
from typing import Callable, List, Optional

from .session import Session
from .utils import pack, unpack

_fs_endpoint = 0x03


@dataclass
class FSInfo:
    name: str
    is_dir: bool
    size: int


async def stat_path(session: Session, path: str, timeout: float = 5.0) -> FSInfo:
    """Return information about the given path."""

    # send command
    cmd = pack("os", 0, path)
    await _send(session, cmd, False, timeout)

    # await reply
    reply = await _receive(session, False, timeout)

    # verify "info" reply
    if len(reply) != 6 or reply[0] != 1:
        raise RuntimeError("invalid reply")

    # unpack "info" reply
    is_dir, size = unpack("oi", reply[1:])

    return FSInfo(path.rsplit("/", 1)[-1], is_dir == 1, size)


async def list_dir(session: Session, dir: str, timeout: float = 5.0) -> List[FSInfo]:
    """Return a list of entries in the given directory."""

    # send command
    cmd = pack("os", 1, dir)
    await _send(session, cmd, False, timeout)

    # prepare infos
    infos = []

    while True:
        # await reply
        reply = await _receive(session, True, timeout)
        if reply is None:
            return infos

        # verify "info" reply
        if len(reply) < 7 or reply[0] != 1:
            raise RuntimeError("invalid reply")

        # unpack "info" reply
        is_dir, size, name = unpack("ois", reply[1:])

        # add info
        infos.append(FSInfo(name, is_dir == 1, size))


async def read_file(
    session: Session,
    file: str,
    report: Optional[Callable[[int], None]] = None,
    timeout: float = 5.0,
) -> bytes:
    """Read the contents of the given file."""

    # stat file
    info = await stat_path(session, file, timeout)

    # prepare data
    data = bytearray()

    # read file in chunks of 5 KB
    while len(data) < info.size:
        # determine length
        length = min(5000, info.size - len(data))

        # read range
        offset = len(data)
        range_ = await read_file_range(
            session,
            file,
            offset,
            length,
            (lambda pos: report(offset + pos)) if report else None,
            timeout,
        )

        # append range
        data += range_

    return bytes(data)


async def read_file_range(
    session: Session,
    file: str,
    offset: int,
    length: int,
    report: Optional[Callable[[int], None]] = None,
    timeout: float = 5.0,
) -> bytes:
    """Read a range of the given file."""

    # send "open" command
    cmd = pack("oos", 2, 0, file)
    await _send(session, cmd, True, timeout)

    # send "read" command
    cmd = pack("oii", 3, offset, length)
    await _send(session, cmd, False, timeout)

    # prepare data
    data = bytearray()

    while True:
        # await reply
        reply = await _receive(session, True, timeout)
        if reply is None:
            break

        # verify "chunk" reply
        if len(reply) <= 5 or reply[0] != 2:
            raise RuntimeError("invalid reply")

        # verify offset
        reply_offset = unpack("i", reply[1:5])[0]
        if reply_offset != offset + len(data):
            raise RuntimeError("invalid offset")

        # append data
        data += reply[5:]

        # report length
        if report:
            report(len(data))

    # send "close" command
    cmd = pack("o", 5)
    await _send(session, cmd, True, timeout)

    return bytes(data)


async def write_file(
    session: Session,
    file: str,
    data: bytes,
    report: Optional[Callable[[int], None]] = None,
    timeout: float = 5.0,
):
    """Write the given data to the given file."""

    # send "create" command (create & truncate)
    cmd = pack("oos", 2, (1 << 0) | (1 << 2), file)
    await _send(session, cmd, True, timeout)

    # get width
    width = session.channel().width()

    # get MTU and subtract overhead
    mtu = await session.get_mtu(timeout) - 6

    # write data in chunks
    num = 0
    offset = 0
    while offset < len(data):
        # determine chunk
        chunk = data[offset : offset + mtu]

        # determine mode
        acked = num % width == 0 or offset + len(chunk) >= len(data)

        # prepare "write" command (sequential or silent & sequential)
        cmd = pack("ooib", 4, 1 << 1 if acked else (1 << 0) | (1 << 1), offset, chunk)

        # send "write" command
        await _send(session, cmd, False, timeout)

        # receive ack or "error" replies
        if acked:
            await _receive(session, True, timeout)

        # increment offset
        offset += len(chunk)

        # report offset
        if report:
            report(offset)

        # increment count
        num += 1

    # send "close" command
    cmd = pack("o", 5)
    await _send(session, cmd, True, timeout)


async def rename_path(session: Session, from_: str, to: str, timeout: float = 5.0):
    """Rename the given path."""

    # send command
    cmd = pack("osos", 6, from_, 0, to)
    await _send(session, cmd, True, timeout)


async def remove_path(session: Session, path: str, timeout: float = 5.0):
    """Remove the given path."""

    # send command
    cmd = pack("os", 7, path)
    await _send(session, cmd, True, timeout)


async def sha256_file(session: Session, file: str, timeout: float = 5.0) -> bytes:
    """Return the SHA-256 hash of the given file."""

    # send command
    cmd = pack("os", 8, file)
    await _send(session, cmd, False, timeout)

    # await reply
    reply = await _receive(session, False, timeout)

    # verify "chunk" reply
    if len(reply) != 33 or reply[0] != 3:
        raise RuntimeError("invalid reply")

    # return hash
    return reply[1:]


async def make_path(session: Session, path: str, timeout: float = 5.0):
    """Create a directory at the given path."""

    # send command
    cmd = pack("os", 9, path)
    await _send(session, cmd, True, timeout)


# Helpers


async def _receive(
    session: Session, expect_ack: bool, timeout: float = 5.0
) -> Optional[bytes]:
    # receive reply
    data, _ = await session.receive(_fs_endpoint, expect_ack, timeout)
    if data is None:
        return None

    # handle errors
    if data[0] == 0:
        raise RuntimeError(f"posix error: {data[1]}")

    return data


async def _send(session: Session, data: bytes, await_ack: bool, timeout: float = 5.0):
    # send command
    await session.send(_fs_endpoint, data, 0)

    # await ack
    if await_ack:
        await _receive(session, True, timeout)
