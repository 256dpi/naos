import asyncio

import pytest

from naos import Message, Session
from naos.ble import BLEDescriptor, BLEDevice, BLEScanner, BLETransport, ble_find
from naos import ble as ble_module
from fake import FakeDeviceTransport


class FakeAdvertisement:
    def __init__(self, local_name=None, rssi=-50):
        self.local_name = local_name
        self.rssi = rssi


class FakeBackendDevice:
    def __init__(self, address, name=None):
        self.address = address
        self.name = name


class FakeScanner:
    """Emulates a bleak scanner that reports devices on demand."""

    def __init__(self, on_detect):
        self.on_detect = on_detect
        self.started = False
        self.stopped = False

    async def start(self):
        self.started = True

    async def stop(self):
        self.stopped = True

    def detect(self, address, name, rssi=-50):
        device = FakeBackendDevice(address, name)
        self.on_detect(device, FakeAdvertisement(name, rssi))
        return device


def install_scanner(monkeypatch, holder):
    def new_scanner(on_detect):
        holder["scanner"] = FakeScanner(on_detect)
        return holder["scanner"]

    monkeypatch.setattr(ble_module, "_new_scanner", new_scanner)


class FakeCharacteristic:
    def __init__(self, uuid):
        self.uuid = uuid


class FakeService:
    def __init__(self, char=None):
        self._char = char

    def get_characteristic(self, uuid):
        if self._char and self._char.uuid == uuid:
            return self._char
        return None


class FakeServices:
    def __init__(self, service=None):
        self._service = service

    def get_service(self, uuid):
        return self._service


class FakeClient:
    """Emulates a bleak client backed by the fake device."""

    def __init__(self, device=None, service=True, char=True):
        self.device = device
        self.connected = False
        self.disconnected = False
        self.notify_started = False
        self.notify_stopped = False
        self.written = []
        self._cb = None

        char_ = FakeCharacteristic(ble_module._char_uuid) if char else None
        self.services = FakeServices(FakeService(char_) if service else None)

    async def connect(self):
        self.connected = True

    async def disconnect(self):
        self.connected = False
        self.disconnected = True

    async def start_notify(self, _uuid, callback):
        self.notify_started = True
        self._cb = callback

    async def stop_notify(self, _uuid):
        self.notify_stopped = True

    async def write_gatt_char(self, uuid, data, response=True):
        # record write
        self.written.append((uuid, bytes(data), response))

        # reply using the fake device
        if self.device:
            msg = Message.parse(bytes(data))
            loop = asyncio.get_running_loop()
            for reply in self.device.handle(msg):
                loop.call_soon(self._cb, 0, bytearray(reply.build()))

    def notify(self, data: bytes):
        self._cb(0, bytearray(data))


async def test_ble_scanner(monkeypatch):
    holder = {}
    install_scanner(monkeypatch, holder)

    detected = []
    scanner = BLEScanner(detected.append)

    async with scanner:
        fake = holder["scanner"]
        assert fake.started

        # devices are sorted by descending signal strength
        fake.detect("AA", "weak", rssi=-80)
        fake.detect("BB", "strong", rssi=-40)
        assert [d.name for d in scanner.devices()] == ["strong", "weak"]
        assert [d.name for d in detected] == ["weak", "strong"]

        # descriptors carry the backend device and are updated in place
        assert scanner.devices()[0].handle.address == "BB"
        fake.detect("BB", "renamed", rssi=-40)
        assert len(scanner.devices()) == 2
        assert scanner.devices()[0].name == "renamed"

        # find returns the strongest device
        assert (await scanner.find()).name == "renamed"

        # find matches by name
        assert (await scanner.find("weak")).name == "weak"

    assert holder["scanner"].stopped

    # only one scan at a time
    await scanner.start()
    with pytest.raises(RuntimeError):
        await scanner.start()
    await scanner.stop()


async def test_ble_scanner_find_waits(monkeypatch):
    holder = {}
    install_scanner(monkeypatch, holder)

    async with BLEScanner() as scanner:
        fake = holder["scanner"]

        # report the device only after find is already waiting
        async def report():
            await asyncio.sleep(0.01)
            fake.detect("AA", "late")

        task = asyncio.create_task(report())
        found = await scanner.find("late", timeout=1)
        await task

        assert found is not None
        assert found.address == "AA"

        # missing devices time out
        assert await scanner.find("other", timeout=0.01) is None


async def test_ble_find(monkeypatch):
    holder = {}
    install_scanner(monkeypatch, holder)

    # a scan that finds nothing stops the scanner and returns None
    assert await ble_find(timeout=0.01) is None
    assert holder["scanner"].stopped


async def test_ble_transport_notify():
    client = FakeClient()
    transport = BLETransport(client, write_delay=0)

    # buffer messages received before start
    transport.handle_notify(0, bytearray(Message(1, 2, b"\x03").build()))
    transport.handle_notify(0, bytearray(b"\x00garbage"))

    received = []
    closed = asyncio.Event()
    transport.start(received.append, closed.set)

    # deliver messages received after start
    transport.handle_notify(0, bytearray(Message(4, 5, None).build()))

    assert len(received) == 2
    assert (received[0].session, received[0].endpoint, received[0].data) == (1, 2, b"\x03")
    assert (received[1].session, received[1].endpoint, received[1].data) == (4, 5, None)
    assert not closed.is_set()


async def test_ble_transport_write():
    client = FakeClient()
    transport = BLETransport(client, write_delay=0)
    transport.start(lambda msg: None, lambda: None)

    msg = Message(1, 2, b"\x03")
    await transport.write(msg)
    assert client.written == [(ble_module._char_uuid, msg.build(), False)]

    await transport.close()
    assert client.notify_stopped
    assert client.disconnected

    # writing after close fails
    with pytest.raises(RuntimeError):
        await transport.write(msg)


async def test_ble_transport_disconnect():
    client = FakeClient()
    transport = BLETransport(client, write_delay=0)

    closed = []
    transport.start(lambda msg: None, lambda: closed.append(True))

    # only the first disconnect is reported
    transport.handle_disconnect()
    transport.handle_disconnect()
    assert closed == [True]


async def test_ble_transport_early_disconnect():
    client = FakeClient()
    transport = BLETransport(client, write_delay=0)

    # disconnect before the channel is created
    transport.handle_disconnect()

    closed = []
    transport.start(lambda msg: None, lambda: closed.append(True))
    assert closed == [True]


async def test_ble_device_open(monkeypatch):
    client = FakeClient(device=FakeDeviceTransport(password=None))
    monkeypatch.setattr(
        ble_module, "_new_client", lambda address, on_disconnect: client
    )

    device = BLEDevice("AA:BB:CC:DD:EE:FF", "test")
    assert device.id() == "ble/AA:BB:CC:DD:EE:FF"
    assert device.type() == "BLE"
    assert device.name() == "test"

    # open channel
    channel = await device.open()
    assert client.connected
    assert client.notify_started
    assert channel.width() == 10
    assert channel.device() is device

    # only one channel at a time
    with pytest.raises(RuntimeError):
        await device.open()

    # run a session
    session = await Session.open(channel)
    await session.ping()
    assert await session.get_mtu() == 120
    await session.end()

    # close channel
    await channel.close()
    assert client.disconnected

    # channel can be opened again
    client.connected = False
    await device.open()


async def test_ble_device_from_descriptor(monkeypatch):
    client = FakeClient()
    targets = []

    def new_client(target, on_disconnect):
        targets.append(target)
        return client

    monkeypatch.setattr(ble_module, "_new_client", new_client)

    # a descriptor from a scan carries the backend device
    handle = FakeBackendDevice("AA:BB:CC:DD:EE:FF", "test")
    device = BLEDevice.from_descriptor(
        BLEDescriptor("AA:BB:CC:DD:EE:FF", "test", -50, handle)
    )
    assert device.name() == "test"

    # the backend device is used to connect without re-discovering the address
    await device.open()
    assert targets == [handle]

    # without a handle the address is used
    await BLEDevice("AA:BB:CC:DD:EE:FF").open()
    assert targets[1] == "AA:BB:CC:DD:EE:FF"


async def test_ble_device_stale_handle(monkeypatch):
    targets = []

    def new_client(target, on_disconnect):
        targets.append(target)

        # fail the first connect to simulate a stale handle
        client = FakeClient()
        if len(targets) == 1:
            async def connect():
                raise RuntimeError("device not found")

            client.connect = connect

        return client

    monkeypatch.setattr(ble_module, "_new_client", new_client)

    handle = FakeBackendDevice("AA:BB:CC:DD:EE:FF", "test")
    device = BLEDevice("AA:BB:CC:DD:EE:FF", "test", handle)

    # the address is used once the handle fails
    channel = await device.open()
    assert targets == [handle, "AA:BB:CC:DD:EE:FF"]

    # the stale handle is not tried again
    await channel.close()
    await device.open()
    assert targets == [handle, "AA:BB:CC:DD:EE:FF", "AA:BB:CC:DD:EE:FF"]


async def test_ble_device_connect_error(monkeypatch):
    def new_client(target, on_disconnect):
        client = FakeClient()

        async def connect():
            raise RuntimeError("out of range")

        client.connect = connect
        return client

    monkeypatch.setattr(ble_module, "_new_client", new_client)

    # without a handle the error is reported directly
    with pytest.raises(RuntimeError, match="out of range"):
        await BLEDevice("AA:BB:CC:DD:EE:FF").open()


async def test_ble_device_missing_service(monkeypatch):
    client = FakeClient(service=False)
    monkeypatch.setattr(
        ble_module, "_new_client", lambda address, on_disconnect: client
    )

    device = BLEDevice("AA:BB:CC:DD:EE:FF")
    assert device.name() == "AA:BB:CC:DD:EE:FF"

    with pytest.raises(RuntimeError, match="missing service"):
        await device.open()
    assert client.disconnected


async def test_ble_device_missing_characteristic(monkeypatch):
    client = FakeClient(char=False)
    monkeypatch.setattr(
        ble_module, "_new_client", lambda address, on_disconnect: client
    )

    device = BLEDevice("AA:BB:CC:DD:EE:FF")

    with pytest.raises(RuntimeError, match="missing characteristic"):
        await device.open()
    assert client.disconnected


async def test_ble_device_disconnect(monkeypatch):
    client = FakeClient(device=FakeDeviceTransport(password=None))
    holder = {}

    def new_client(address, on_disconnect):
        holder["on_disconnect"] = on_disconnect
        return client

    monkeypatch.setattr(ble_module, "_new_client", new_client)

    device = BLEDevice("AA:BB:CC:DD:EE:FF")
    channel = await device.open()

    # simulate device disconnect
    holder["on_disconnect"]()
    await asyncio.wait_for(channel.wait_closed(), 1)
    assert client.disconnected
