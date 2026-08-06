import hashlib

import pytest

from naos import (
    Channel,
    Session,
    list_dir,
    make_path,
    read_file,
    read_file_range,
    remove_path,
    rename_path,
    sha256_file,
    stat_path,
    write_file,
)

from fake import FakeDeviceTransport


async def open_session():
    transport = FakeDeviceTransport()
    channel = Channel(transport, None, 1)
    session = await Session.open(channel)
    return transport, channel, session


async def test_fs_stat():
    transport, channel, session = await open_session()

    info = await stat_path(session, "/data/test.txt")
    assert info.name == "test.txt"
    assert not info.is_dir
    assert info.size == 11

    info = await stat_path(session, "/data")
    assert info.is_dir

    with pytest.raises(RuntimeError, match="posix error: 2"):
        await stat_path(session, "/missing")

    await channel.close()


async def test_fs_list():
    transport, channel, session = await open_session()

    infos = await list_dir(session, "/data")
    assert len(infos) == 1
    assert infos[0].name == "test.txt"
    assert infos[0].size == 11

    await channel.close()


async def test_fs_read():
    transport, channel, session = await open_session()

    # multi-chunk read (chunk size 40)
    transport.files["/data/big.bin"] = bytes(range(256)) * 4

    reports = []
    data = await read_file(session, "/data/big.bin", reports.append)
    assert data == transport.files["/data/big.bin"]
    assert reports[-1] == 1024

    # ranged read
    data = await read_file_range(session, "/data/big.bin", 100, 20)
    assert data == transport.files["/data/big.bin"][100:120]

    await channel.close()


async def test_fs_write():
    transport, channel, session = await open_session()

    # multi-chunk write (MTU 120 - 6 overhead = 114 byte chunks)
    payload = bytes(range(256)) * 2
    reports = []
    await write_file(session, "/data/out.bin", payload, reports.append)
    assert transport.files["/data/out.bin"] == payload
    assert reports[-1] == 512

    await channel.close()


async def test_fs_manage():
    transport, channel, session = await open_session()

    await rename_path(session, "/data/test.txt", "/data/renamed.txt")
    assert "/data/renamed.txt" in transport.files

    digest = await sha256_file(session, "/data/renamed.txt")
    assert digest == hashlib.sha256(b"hello world").digest()

    await remove_path(session, "/data/renamed.txt")
    assert "/data/renamed.txt" not in transport.files

    await make_path(session, "/stuff")
    assert (await stat_path(session, "/stuff")).is_dir

    await channel.close()
