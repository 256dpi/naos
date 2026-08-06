import asyncio
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

        # unknown endpoint
        return [Message(msg.session, 0xFE, bytes([3]))]

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
