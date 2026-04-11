import { Device, Channel, Message, Transport } from "./device";

const svcUUID = "632fba1b-4861-4e4f-8103-ffee9d5033b5";
const charUUID = "0360744b-a61b-00ad-c945-37f3634130f3";

export async function bleRequest(bt?: Bluetooth): Promise<Device | null> {
  // use provided or global bluetooth instance
  if (!bt) {
    bt = navigator.bluetooth;
  }

  // request device
  let dev: BluetoothDevice | null;
  try {
    dev = await bt.requestDevice({
      filters: [{ services: [svcUUID] }],
    });
  } catch (err) {
    // ignore
  }
  if (!dev) {
    return null;
  }

  return new BLEDevice(dev);
}

export class BLEDevice implements Device {
  private dev: BluetoothDevice;
  private svc: BluetoothRemoteGATTService | null = null;
  private char: BluetoothRemoteGATTCharacteristic | null = null;
  private ch: Channel | null = null;

  constructor(dev: BluetoothDevice) {
    // store device
    this.dev = dev;

    // close open channel and clear stale GATT state if disconnected
    this.dev.addEventListener("gattserverdisconnected", () => {
      this.svc = null;
      this.char = null;
      if (this.ch) {
        this.ch.close().catch(() => {});
        this.ch = null;
      }
    });
  }

  id() {
    return "ble/" + this.dev.id;
  }

  type() {
    return "BLE";
  }

  name() {
    return this.dev.name || "Unnamed";
  }

  async open(): Promise<Channel> {
    // check channel
    if (this.ch) {
      throw new Error("channel already open");
    }

    // connect, if not connected already
    if (!this.dev.gatt.connected) {
      await this.dev.gatt.connect();
    }

    // get service and characteristic if not available
    if (!this.svc) {
      this.svc = await this.dev.gatt.getPrimaryService(svcUUID);
      this.char = await this.svc.getCharacteristic(charUUID);
      if (!this.char) {
        throw new Error("missing characteristic");
      }
    }

    let handler: (() => void) | null = null;
    let disconnect: (() => void) | null = null;

    const transport: Transport = {
      start: async (onData, onClose) => {
        handler = () => {
          const value = this.char.value;
          const msg = Message.parse(
            new Uint8Array(
              value.buffer.slice(
                value.byteOffset,
                value.byteOffset + value.byteLength
              )
            )
          );
          if (msg) {
            onData(msg);
          }
        };
        disconnect = () => {
          onClose();
        };

        this.char.addEventListener("characteristicvaluechanged", handler);
        this.dev.addEventListener("gattserverdisconnected", disconnect);
        await this.char.startNotifications();
      },
      write: async (msg: Message) => {
        await this.char.writeValueWithoutResponse(msg.build() as BufferSource);
      },
      close: async () => {
        if (handler) {
          this.char.removeEventListener("characteristicvaluechanged", handler);
        }
        if (disconnect) {
          this.dev.removeEventListener("gattserverdisconnected", disconnect);
        }
        if (this.char) {
          await this.char.stopNotifications().catch(() => {});
        }
      },
    };

    this.ch = new Channel(transport, this, 10, () => {
      this.ch = null;
    });
    return this.ch;
  }
}
