import asyncio
import hashlib
import struct

from naos import Message, Transport


class FakeDeviceTransport(Transport):
    """Emulates the device side of the messaging protocol."""

    def __init__(self, password="secret", mtu=120):
        self.password = password
        self.mtu = mtu
        self.locked = password is not None
        self.closed = False
        self.next_sid = 1
        self.on_data = None
        self.on_close = None
        self.params = {
            1: {"name": "app-name", "type": 1, "mode": 1 << 1, "value": b"test", "age": 10},
            2: {"name": "counter", "type": 3, "mode": 1 << 2, "value": b"42", "age": 20},
        }
        self.files = {"/data/test.txt": b"hello world"}
        self.dirs = {"/data"}
        self.open_file = None
        self.read_chunk_size = 40
        self.metrics = {
            0: {
                "name": "co2",
                "kind": 1,
                "type": 2,
                "layout": [("room", ["lab", "office"])],
                "values": struct.pack("<2d", 412.5, 587.0),
            },
            1: {
                "name": "uptime",
                "kind": 0,
                "type": 0,
                "layout": [],
                "values": struct.pack("<1i", 4711),
            },
        }
        self.time_ms = 1700000000000
        self.time_offset = 3600

    def start(self, on_data, on_close):
        self.on_data = on_data
        self.on_close = on_close

    async def write(self, msg: Message):
        if self.closed:
            raise RuntimeError("closed")
        loop = asyncio.get_running_loop()
        for reply in self.handle(msg):
            loop.call_soon(self.on_data, reply)

    async def close(self):
        self.closed = True

    def handle(self, msg: Message):
        # session open
        if msg.session == 0 and msg.endpoint == 0x0:
            sid = self.next_sid
            self.next_sid += 1
            return [Message(sid, 0x0, msg.data)]

        # ping
        if msg.endpoint == 0xFE:
            return [Message(msg.session, 0xFE, bytes([1]))]

        # session management
        if msg.endpoint == 0xFD:
            cmd = msg.data[0]
            if cmd == 0:  # status
                return [Message(msg.session, 0xFD, bytes([1 if self.locked else 0]))]
            if cmd == 1:  # unlock
                if msg.data[1:].decode() == self.password:
                    self.locked = False
                    return [Message(msg.session, 0xFD, bytes([1]))]
                return [Message(msg.session, 0xFD, bytes([0]))]
            if cmd == 2:  # MTU
                return [Message(msg.session, 0xFD, struct.pack("<H", self.mtu))]

        # session end
        if msg.endpoint == 0xFF:
            return [Message(msg.session, 0xFF, None)]

        # params
        if msg.endpoint == 0x01:
            return self.handle_params(msg)

        # fs
        if msg.endpoint == 0x03:
            return self.handle_fs(msg)

        # metrics
        if msg.endpoint == 0x05:
            return self.handle_metrics(msg)

        # time
        if msg.endpoint == 0x09:
            return self.handle_time(msg)

        # unknown endpoint
        return [Message(msg.session, 0xFE, bytes([3]))]

    def handle_fs(self, msg: Message):
        cmd = msg.data[0]
        ack = Message(msg.session, 0xFE, bytes([1]))

        def error(errno):
            return [Message(msg.session, 0x03, bytes([0, errno]))]

        if cmd == 0:  # stat
            path = msg.data[1:].decode()
            if path in self.dirs:
                return [Message(msg.session, 0x03, struct.pack("<BBI", 1, 1, 0))]
            if path not in self.files:
                return error(2)  # ENOENT
            size = len(self.files[path])
            return [Message(msg.session, 0x03, struct.pack("<BBI", 1, 0, size))]
        if cmd == 1:  # list dir
            dir_ = msg.data[1:].decode()
            replies = [
                Message(
                    msg.session,
                    0x03,
                    struct.pack("<BBI", 1, 0, len(data))
                    + path[len(dir_) :].lstrip("/").encode(),
                )
                for path, data in sorted(self.files.items())
                if path.startswith(dir_ + "/")
            ]
            return replies + [ack]
        if cmd == 2:  # open
            flags, path = msg.data[1], msg.data[2:].decode()
            if flags & (1 << 0):  # create
                self.files.setdefault(path, b"")
            if flags & (1 << 2):  # truncate
                self.files[path] = b""
            if path not in self.files:
                return error(2)
            self.open_file = path
            return [ack]
        if cmd == 3:  # read
            offset, length = struct.unpack_from("<II", msg.data, 1)
            data = self.files[self.open_file][offset : offset + length]
            replies = [
                Message(
                    msg.session,
                    0x03,
                    struct.pack("<BI", 2, offset + pos)
                    + data[pos : pos + self.read_chunk_size],
                )
                for pos in range(0, len(data), self.read_chunk_size)
            ]
            return replies + [ack]
        if cmd == 4:  # write
            flags, offset = msg.data[1], struct.unpack_from("<I", msg.data, 2)[0]
            data = self.files[self.open_file]
            self.files[self.open_file] = data[:offset] + msg.data[6:]
            return [] if flags & (1 << 0) else [ack]  # silent
        if cmd == 5:  # close
            self.open_file = None
            return [ack]
        if cmd == 6:  # rename
            from_, _, to = msg.data[1:].partition(b"\x00")
            self.files[to.decode()] = self.files.pop(from_.decode())
            return [ack]
        if cmd == 7:  # remove
            del self.files[msg.data[1:].decode()]
            return [ack]
        if cmd == 8:  # sha256
            digest = hashlib.sha256(self.files[msg.data[1:].decode()]).digest()
            return [Message(msg.session, 0x03, bytes([3]) + digest)]
        if cmd == 9:  # mkdir
            self.dirs.add(msg.data[1:].decode())
            return [ack]

        return [Message(msg.session, 0xFE, bytes([2]))]

    def handle_metrics(self, msg: Message):
        cmd = msg.data[0]
        ack = Message(msg.session, 0xFE, bytes([1]))

        if cmd == 0:  # list
            replies = [
                Message(
                    msg.session,
                    0x05,
                    bytes([ref, m["kind"], m["type"], len(m["values"])])
                    + m["name"].encode(),
                )
                for ref, m in self.metrics.items()
            ]
            return replies + [ack]
        if cmd == 1:  # describe
            metric = self.metrics[msg.data[1]]
            replies = []
            for num, (key, values) in enumerate(metric["layout"]):
                replies.append(
                    Message(msg.session, 0x05, bytes([0, num]) + key.encode())
                )
                for sub, value in enumerate(values):
                    replies.append(
                        Message(msg.session, 0x05, bytes([1, num, sub]) + value.encode())
                    )
            return replies + [ack]
        if cmd == 2:  # read
            return [Message(msg.session, 0x05, self.metrics[msg.data[1]]["values"])]

        return [Message(msg.session, 0xFE, bytes([2]))]

    def handle_time(self, msg: Message):
        cmd = msg.data[0]

        if cmd == 0:  # get time
            return [Message(msg.session, 0x09, struct.pack("<q", self.time_ms))]
        if cmd == 1:  # set time
            self.time_ms = struct.unpack_from("<q", msg.data, 1)[0]
            return [Message(msg.session, 0xFE, bytes([1]))]
        if cmd == 2:  # get info
            return [Message(msg.session, 0x09, struct.pack("<i", self.time_offset))]

        return [Message(msg.session, 0xFE, bytes([2]))]

    def handle_params(self, msg: Message):
        cmd = msg.data[0]
        ack = Message(msg.session, 0xFE, bytes([1]))

        def by_name(name):
            for ref, param in self.params.items():
                if param["name"] == name:
                    return ref, param
            return None, None

        if cmd == 0:  # get by name
            _, param = by_name(msg.data[1:].decode())
            return [Message(msg.session, 0x01, param["value"])]
        if cmd == 1:  # set by name
            name, _, value = msg.data[1:].partition(b"\x00")
            _, param = by_name(name.decode())
            param["value"] = value
            param["age"] += 1
            return [ack]
        if cmd == 2:  # list
            replies = [
                Message(
                    msg.session,
                    0x01,
                    bytes([ref, p["type"], p["mode"]]) + p["name"].encode(),
                )
                for ref, p in self.params.items()
            ]
            return replies + [ack]
        if cmd == 3:  # read by ref
            return [Message(msg.session, 0x01, self.params[msg.data[1]]["value"])]
        if cmd == 4:  # write by ref
            param = self.params[msg.data[1]]
            param["value"] = msg.data[2:]
            param["age"] += 1
            return [ack]
        if cmd == 5:  # collect
            map_, since = struct.unpack_from("<QQ", msg.data, 1)
            replies = [
                Message(
                    msg.session,
                    0x01,
                    struct.pack("<BQ", ref, p["age"]) + p["value"],
                )
                for ref, p in self.params.items()
                if map_ & (1 << ref) and p["age"] > since
            ]
            return replies + [ack]
        if cmd == 6:  # clear by ref
            param = self.params[msg.data[1]]
            param["value"] = b""
            param["age"] += 1
            return [ack]

        return [Message(msg.session, 0xFE, bytes([2]))]
