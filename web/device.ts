import { Queue as WorkQueue } from "async-await-queue";

import { concat, toBuffer, toString } from "./utils";
import { Channel, Queue } from "./channel";

async function write(char, data, confirm = true) {
  if (typeof data === "string") {
    data = toBuffer(data);
  }
  if (confirm) {
    await char.writeValueWithResponse(data);
  } else {
    await char.writeValueWithoutResponse(data);
  }
}

async function read(char) {
  const value = await char.readValue();
  return toString(value);
}

export const UUIDs = {
  service: "632fba1b-4861-4e4f-8103-ffee9d5033b5",
  lockChar: "f7a5fba4-4084-239b-684d-07d5902eb591",
  listChar: "ac2289d1-231b-b78b-df48-7d951a6ea665",
  selectChar: "cfc9706d-406f-ccbe-4240-f88d6ed4bacd",
  valueChar: "01ca5446-8ee1-7e99-2041-6884b01e71b3",
  updateChar: "87bffdcf-0704-22a2-9c4a-7a61bc8c1726",
  flashChar: "6c114da1-9aa9-1687-5341-a1fe4c991390",
  msgChar: "0360744b-a61b-00ad-c945-37f3634130f3",
};

export const Types = {
  Bool: "b",
  Long: "l",
  Double: "d",
  String: "s",
  Action: "a",  
};

export const Modes = {
  Volatile: "v",
  System: "s",
  Application: "a",
  Public: "p",
  Locked: "l",
};

export interface Param {
  name: string;
  type: string;
  mode: string;
}

export interface ManagedDeviceOptions {
  subscribe: boolean;
  autoUpdate: boolean;
}

export class ManagedDevice extends EventTarget {
  wq = new WorkQueue();

  options: ManagedDeviceOptions;
  device: BluetoothDevice;
  service: BluetoothRemoteGATTService;
  lockChar: BluetoothRemoteGATTCharacteristic;
  listChar: BluetoothRemoteGATTCharacteristic;
  selectChar: BluetoothRemoteGATTCharacteristic;
  valueChar: BluetoothRemoteGATTCharacteristic;
  updateChar: BluetoothRemoteGATTCharacteristic;
  flashChar: BluetoothRemoteGATTCharacteristic;
  msgChar: BluetoothRemoteGATTCharacteristic;

  protected = false;
  locked = false;
  parameters: Param[] = [];
  updated = new Set<string>();
  cache = {};
  timer: number;
  channel: Channel | null;

  constructor(
    device: BluetoothDevice,
    options: ManagedDeviceOptions = {
      subscribe: false,
      autoUpdate: false,
    }
  ) {
    super();

    // set device
    this.device = device;

    // set options
    this.options = {
      ...{
        subscribe: true,
        autoUpdate: true,
      },
      ...options,
    };

    // handle disconnects
    this.device.addEventListener("gattserverdisconnected", () => {
      this.dispatchEvent(new CustomEvent("disconnected"));
    });
  }

  /* Connection */

  get name(): string {
    return this.device.name;
  }

  get connected(): boolean {
    return this.device.gatt.connected;
  }

  async connect(): Promise<void> {
    await this.wq.run(async () => {
      // connect
      await this.device.gatt.connect();

      // get service
      this.service = await this.device.gatt.getPrimaryService(UUIDs.service);

      // get characteristics
      this.lockChar = await this.service.getCharacteristic(UUIDs.lockChar);
      this.listChar = await this.service.getCharacteristic(UUIDs.listChar);
      this.selectChar = await this.service.getCharacteristic(UUIDs.selectChar);
      this.valueChar = await this.service.getCharacteristic(UUIDs.valueChar);
      this.updateChar = await this.service.getCharacteristic(UUIDs.updateChar);
      this.flashChar = await this.service.getCharacteristic(UUIDs.flashChar);
      try {
        this.msgChar = await this.service.getCharacteristic(UUIDs.msgChar);
      } catch (_) {}

      // handle updates
      this.updateChar.addEventListener(
        "characteristicvaluechanged",
        (event) => {
          const name = toString(this.updateChar.value.buffer);
          this.updated.add(name);
          this.dispatchEvent(new CustomEvent("changed", { detail: name }));
        }
      );

      // subscribe to updates
      if (this.options.subscribe) {
        await this.updateChar.startNotifications();
      }

      // create timer
      if (this.options.autoUpdate) {
        this.timer = setInterval(() => {
          this.wq.run(async () => {
            // get and replace updated
            const updated = this.updated;
            this.updated = new Set();

            // skip empty sets
            if (updated.size === 0) {
              return;
            }

            // update values
            const updates = {};
            for (let name of updated) {
              await write(this.selectChar, name);
              this.cache[name] = await read(this.valueChar);
              updates[name] = this.cache[name];
            }

            // dispatch event
            this.dispatchEvent(new CustomEvent("updated", { detail: updates }));
          });
        }, 1000);
      }

      // subscribe and handle messages if available
      if (this.msgChar) {
        this.msgChar.addEventListener("characteristicvaluechanged", (event) => {
          const data = this.msgChar.value;
          this.dispatchEvent(new CustomEvent("message", { detail: data }));
        });
        await this.msgChar.startNotifications();
      }

      // dispatch event
      this.dispatchEvent(new CustomEvent("connected"));
    });
  }

  async refresh() {
    await this.wq.run(async () => {
      // check state
      if (!this.connected) {
        throw new Error("not connected");
      }

      // update locked
      this.locked = (await read(this.lockChar)) === "locked";

      // save if this device is protected
      if (this.locked) {
        this.protected = true;
      }

      // read system parameters
      await write(this.listChar, "system");
      let system = await read(this.listChar);

      // read application parameters
      await write(this.listChar, "application");
      let application = await read(this.listChar);

      // read list
      let list = system + "," + application;

      // parse parameters
      this.parameters = list
        .split(",")
        .map((str) => {
          const seg = str.split(":");
          if (seg.length !== 3) {
            return null;
          }
          return {
            name: seg[0],
            type: seg[1],
            mode: seg[2],
          };
        })
        .filter((item) => !!item);

      // read all parameters
      for (let param of this.parameters) {
        await write(this.selectChar, param.name);
        this.cache[param.name] = await read(this.valueChar);
      }
    });
  }

  async unlock(password: string) {
    return await this.wq.run(async () => {
      // check state
      if (!this.connected) {
        throw new Error("not connected");
      }

      // write lock
      await write(this.lockChar, password);

      // read lock
      const lock = await read(this.lockChar);

      // update state
      this.locked = lock === "locked";

      return lock === "unlocked";
    });
  }

  async read(name: string) {
    return await this.wq.run(async () => {
      // check state
      if (!this.connected) {
        throw new Error("not connected");
      }

      // select parameter
      await write(this.selectChar, name);

      // read parameter
      const value = await read(this.valueChar);

      // update cache
      this.cache[name] = value;

      return value;
    });
  }

  async write(name: string, value: Uint8Array) {
    return await this.wq.run(async () => {
      // check state
      if (!this.connected) {
        throw new Error("not connected");
      }

      // select parameter
      await write(this.selectChar, name);

      // write parameter
      await write(this.valueChar, value);

      // update cache
      this.cache[name] = value;
    });
  }

  async quickWrite(name: string, values: Uint8Array, confirm = false) {
    return await this.wq.run(async () => {
      // check state
      if (!this.connected) {
        throw new Error("not connected");
      }

      // select parameter
      await write(this.selectChar, name, confirm);

      // write value
      for (const value of values) {
        // write parameter
        await write(this.valueChar, value, confirm);

        // update cache
        this.cache[name] = value;
      }
    });
  }

  async flash(data: Uint8Array, progress) {
    return await this.wq.run(async () => {
      // check state
      if (!this.connected) {
        throw new Error("not connected");
      }

      // subscribe signal
      await this.flashChar.startNotifications();

      // prepare signal
      const signal = new Promise((resolve) => {
        const listener = (event) => {
          const res = toString(event.target.value);
          this.flashChar.removeEventListener(
            "characteristicvaluechanged",
            listener
          );
          resolve(res === "1");
        };
        this.flashChar.addEventListener("characteristicvaluechanged", listener);
      });

      // begin update
      await write(this.flashChar, `b${data.byteLength}`, true);

      // await signal
      await signal;

      // unsubscribe signal
      await this.flashChar.stopNotifications();

      // get start
      const start = Date.now();

      // call progress
      if (progress) {
        progress({
          done: 0,
          total: data.byteLength,
          rate: 0,
          percent: 0,
        });
      }

      // write update
      let chunks = 0;
      for (let i = 0; i < data.byteLength; i += 500) {
        const buf = concat(toBuffer("w"), data.slice(i, i + 500));
        await write(this.flashChar, buf, chunks % 5 === 0);
        chunks++;
        if (progress) {
          progress({
            done: i,
            total: data.byteLength,
            rate: (i + buf.byteLength) / ((Date.now() - start) / 1000),
            percent: (100 / data.byteLength) * (i + buf.byteLength),
          });
        }
      }

      // call progress
      if (progress) {
        progress({
          done: data.byteLength,
          total: data.byteLength,
          rate: data.byteLength / ((Date.now() - start) / 1000),
          percent: 100,
        });
      }

      // finish update
      await write(this.flashChar, "f", true);
    });
  }

  getChannel(): Channel {
    // check characteristic
    if (!this.msgChar) {
      return;
    }

    // check channel
    if (this.channel) {
      return this.channel;
    }

    // create list
    const subscribers: Queue[] = [];

    // prepare handler
    const handler = (event) => {
      for (let queue of subscribers) {
        const data = event.detail as DataView;
        queue.push(new Uint8Array(data.buffer));
      }
    };

    // subscribe to messages
    this.addEventListener("message", handler);

    // create channel
    this.channel = {
      name: () => "ble",
      subscribe: (q: Queue) => {
        subscribers.push(q);
      },
      unsubscribe(queue: Queue) {
        const index = subscribers.indexOf(queue);
        if (index >= 0) {
          subscribers.splice(index, 1);
        }
      },
      write: async (data: Uint8Array) => {
        await write(this.msgChar, data, false);
      },
      close: () => {
        this.removeEventListener("message", handler);
        this.channel = null;
      },
    };

    return this.channel;
  }

  async disconnect() {
    return await this.wq.run(async () => {
      // lock again if protected
      if (this.protected) {
        this.locked = true;
      }

      // disconnect
      await this.device.gatt.disconnect();

      // dispatch event
      this.dispatchEvent(new CustomEvent("disconnected"));
    });
  }
}
