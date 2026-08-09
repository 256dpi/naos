from __future__ import annotations

import asyncio
import sys
from typing import Any, Callable, List, Optional

from .device import Channel, Device, Message, Transport

_service_uuid = "632fba1b-4861-4e4f-8103-ffee9d5033b5"
_char_uuid = "0360744b-a61b-00ad-c945-37f3634130f3"

# number of concurrent sessions supported by the device
_channel_width = 10

# delay after writes to not overwhelm the BlueZ stack
_write_delay = 0.005 if sys.platform.startswith("linux") else 0.0


def _import_bleak():
    try:
        import bleak
    except ImportError as exc:  # pragma: no cover
        raise RuntimeError(
            "BLE support requires bleak: pip install 'naos[ble]'"
        ) from exc

    return bleak


def _new_client(address: str, on_disconnect: Callable[[], None]) -> Any:
    # create client
    bleak = _import_bleak()
    return bleak.BleakClient(
        address, disconnected_callback=lambda _client: on_disconnect()
    )


class BLEDescriptor:
    """BLEDescriptor describes a discovered BLE device."""

    __slots__ = ("address", "name", "rssi")

    def __init__(self, address: str, name: Optional[str], rssi: Optional[int]):
        self.address = address
        self.name = name
        self.rssi = rssi

    def __repr__(self):
        return f"BLEDescriptor(address={self.address}, name={self.name}, rssi={self.rssi})"


async def ble_scan(duration: float = 5.0) -> List[BLEDescriptor]:
    """Scan for NAOS devices for the specified duration."""

    # scan for devices advertising the service
    bleak = _import_bleak()
    found = await bleak.BleakScanner.discover(
        timeout=duration, service_uuids=[_service_uuid], return_adv=True
    )

    # collect descriptors
    descriptors: List[BLEDescriptor] = []
    for device, adv in found.values():
        # skip devices that do not advertise the service
        uuids = [uuid.lower() for uuid in (adv.service_uuids or [])]
        if uuids and _service_uuid not in uuids:
            continue

        descriptors.append(
            BLEDescriptor(device.address, adv.local_name or device.name, adv.rssi)
        )

    # sort by descending signal strength
    descriptors.sort(key=lambda d: d.rssi if d.rssi is not None else -128, reverse=True)

    return descriptors


async def ble_find(
    name: Optional[str] = None, duration: float = 5.0
) -> Optional[BLEDescriptor]:
    """Return the first best available NAOS device, optionally matching the
    provided name."""

    # scan devices
    devices = await ble_scan(duration)

    # find device
    for device in devices:
        if name is None or device.name == name:
            return device

    return None


class BLEDevice(Device):
    def __init__(self, address: str, name: Optional[str] = None):
        # store address and name
        self._address = address
        self._name = name
        self._ch: Optional[Channel] = None

    @classmethod
    def from_descriptor(cls, descriptor: BLEDescriptor) -> BLEDevice:
        """Create a device from a scan result."""

        return cls(descriptor.address, descriptor.name)

    def id(self) -> str:
        return f"ble/{self._address}"

    def type(self) -> str:
        return "BLE"

    def name(self) -> str:
        return self._name or self._address

    async def open(self) -> Channel:
        # check channel
        if self._ch:
            raise RuntimeError("channel already open")

        # prepare transport reference for the disconnect handler
        ref: List[Optional[BLETransport]] = [None]

        def on_disconnect():
            if ref[0]:
                ref[0].handle_disconnect()

        # create client
        client = _new_client(self._address, on_disconnect)

        # connect to device
        await client.connect()

        # create transport
        transport = BLETransport(client)
        ref[0] = transport

        try:
            # check service
            service = client.services.get_service(_service_uuid)
            if service is None:
                raise RuntimeError("missing service")

            # check characteristic
            if service.get_characteristic(_char_uuid) is None:
                raise RuntimeError("missing characteristic")

            # subscribe to characteristic
            await client.start_notify(_char_uuid, transport.handle_notify)
        except Exception:
            await transport.close()
            raise

        # create channel
        self._ch = Channel(transport, self, _channel_width, self._clear)
        return self._ch

    def _clear(self):
        self._ch = None


class BLETransport(Transport):
    def __init__(self, client: Any, write_delay: float = _write_delay):
        self._client = client
        self._delay = write_delay
        self._lock = asyncio.Lock()
        self._closed = False
        self._pending: List[Message] = []
        self._on_data: Optional[Callable[[Message], None]] = None
        self._on_close: Optional[Callable[[], None]] = None

    def start(self, on_data: Callable[[Message], None], on_close: Callable[[], None]):
        # store handlers
        self._on_data = on_data
        self._on_close = on_close

        # flush messages received before the channel was created
        pending, self._pending = self._pending, []
        for msg in pending:
            on_data(msg)

        # report already handled disconnect
        if self._closed:
            on_close()

    def handle_notify(self, _char: Any, data: bytearray):
        # parse message
        msg = Message.parse(bytes(data))
        if not msg:
            return

        # buffer message until the channel is created
        if not self._on_data:
            self._pending.append(msg)
            return

        self._on_data(msg)

    def handle_disconnect(self):
        # check state
        if self._closed:
            return

        # mark as closed
        self._closed = True

        # notify channel
        if self._on_close:
            self._on_close()

    async def write(self, msg: Message):
        async with self._lock:
            # check state
            if self._closed:
                raise RuntimeError("transport closed")

            # write to characteristic
            await self._client.write_gatt_char(_char_uuid, msg.build(), response=False)

            # pace writes if requested
            if self._delay:
                await asyncio.sleep(self._delay)

    async def close(self):
        # mark as closed
        self._closed = True

        # unsubscribe from characteristic
        try:
            await self._client.stop_notify(_char_uuid)
        except Exception:
            pass  # device already disconnected

        # disconnect from device
        try:
            await self._client.disconnect()
        except Exception:
            pass  # device already disconnected
