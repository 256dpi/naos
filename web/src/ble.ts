import { Device, Channel, Queue, QueueList } from "./device";

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

    // close open chanel if disconnected
    this.dev.addEventListener("gattserverdisconnected", () => {
      if (this.ch) {
        this.ch.close();
        this.ch = null;
      }
    });
  }

  id() {
    return "ble/" + this.dev.id;
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

    // create list
    const subscribers = new QueueList();

    // prepare handler
    const handler = () => {
      const data = new Uint8Array(this.char.value.buffer);
      subscribers.dispatch(data);
    };

    // subscribe to messages
    this.char.addEventListener("characteristicvaluechanged", handler);
    await this.char.startNotifications();

    // prepare flag
    let closed = false;

    // create channel
    this.ch = {
      name: () => "ble",
      valid: () => {
        return this.dev.gatt.connected && !closed;
      },
      width() {
        return 10;
      },
      subscribe: (q: Queue) => {
        subscribers.add(q);
      },
      unsubscribe(queue: Queue) {
        subscribers.drop(queue);
      },
      write: async (data: Uint8Array) => {
        await this.char.writeValueWithoutResponse(data as BufferSource);
      },
      close: async () => {
        this.char.removeEventListener("characteristicvaluechanged", handler);
        this.ch = null;
        closed = true;
      },
    };

    return this.ch;
  }
}
