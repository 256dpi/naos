import pytest

from naos import pack, random_handle, unpack


def test_pack():
    assert pack("o", 7) == b"\x07"
    assert pack("h", 0x1234) == b"\x34\x12"
    assert pack("i", 0x12345678) == b"\x78\x56\x34\x12"
    assert pack("q", 1) == b"\x01" + b"\x00" * 7
    assert pack("s", "foo") == b"foo"
    assert pack("b", b"\x01\x02") == b"\x01\x02"
    assert pack("os", 1, "secret") == b"\x01secret"

    with pytest.raises(ValueError):
        pack("x", 1)
    with pytest.raises(ValueError):
        pack("o", 1, 2)


def test_unpack():
    assert unpack("o", b"\x07") == [7]
    assert unpack("h", b"\x34\x12") == [0x1234]
    assert unpack("i", b"\x78\x56\x34\x12") == [0x12345678]
    assert unpack("q", b"\x01" + b"\x00" * 7) == [1]
    assert unpack("s", b"foo") == ["foo"]
    assert unpack("s", b"foo\x00bar") == ["foo"]
    assert unpack("ss", b"foo\x00bar") == ["foo", "bar"]
    assert unpack("ob", b"\x01\x02\x03") == [1, b"\x02\x03"]

    with pytest.raises(ValueError):
        unpack("x", b"")


def test_roundtrip():
    buf = pack("ohiqs", 1, 2, 3, 4, "hello")
    assert unpack("ohiqs", buf) == [1, 2, 3, 4, "hello"]


def test_random_handle():
    handle = random_handle(16)
    assert len(handle) == 16
    assert handle.isalnum()
    assert handle != random_handle(16)
