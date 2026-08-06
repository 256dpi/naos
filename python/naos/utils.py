import secrets
import string
import struct

_HANDLE_CHARS = string.ascii_uppercase + string.ascii_lowercase + string.digits


def random_handle(length: int) -> str:
    """Return a random alphanumeric handle of the given length."""

    return "".join(secrets.choice(_HANDLE_CHARS) for _ in range(length))


def pack(fmt: str, *args) -> bytes:
    """Pack the arguments according to the format string.

    Format codes: "s" UTF-8 string, "b" bytes, "o" uint8, "h" uint16,
    "i" uint32 and "q" uint64. Numbers are encoded in little-endian order.
    """

    # check lengths
    if len(fmt) != len(args):
        raise ValueError("invalid format")

    # write arguments
    buffer = bytearray()
    for code, arg in zip(fmt, args):
        if code == "s":
            buffer += arg.encode()
        elif code == "b":
            buffer += bytes(arg)
        elif code == "o":
            buffer += struct.pack("<B", arg)
        elif code == "h":
            buffer += struct.pack("<H", arg)
        elif code == "i":
            buffer += struct.pack("<I", arg)
        elif code == "q":
            buffer += struct.pack("<Q", arg)
        else:
            raise ValueError("invalid format")

    return bytes(buffer)


def unpack(fmt: str, buffer: bytes) -> list:
    """Unpack the buffer according to the format string.

    Format codes: "s" null-terminated UTF-8 string, "b" remaining bytes,
    "o" uint8, "h" uint16, "i" uint32 and "q" uint64. Numbers are decoded in
    little-endian order.
    """

    # read arguments
    result = []
    pos = 0
    for code in fmt:
        if code == "s":
            end = buffer.find(0, pos)
            if end == -1:
                end = len(buffer)
            result.append(buffer[pos:end].decode())
            pos = end + 1
        elif code == "b":
            result.append(buffer[pos:])
            pos = len(buffer)
        elif code == "o":
            result.append(buffer[pos])
            pos += 1
        elif code == "h":
            result.append(struct.unpack_from("<H", buffer, pos)[0])
            pos += 2
        elif code == "i":
            result.append(struct.unpack_from("<I", buffer, pos)[0])
            pos += 4
        elif code == "q":
            result.append(struct.unpack_from("<Q", buffer, pos)[0])
            pos += 8
        else:
            raise ValueError(f"invalid format code: {code}")

    return result
