import { Queue } from "async-await-queue";

import { Session } from "./session.js";
import { AsyncQueue, concat, toBuffer, toString } from "./utils.js";

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

export class Device extends EventTarget {
  queue = new Queue();

  options;
  device;
  service;
  lockChar;
  listChar;
  selectChar;
  valueChar;
  updateChar;
  flashChar;
  msgChar;

  protected = false;
  locked = false;
  parameters;
  updated = new Set();
  cache = {};
  timer;

  constructor(device, options = {}) {
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

  get name() {
    return this.device.name;
  }

  get connected() {
    return this.device.gatt.connected;
  }

  async connect() {
    await this.queue.run(async () => {
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
          const name = toString(event.target.value);
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
          this.queue.run(async () => {
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
          const data = event.target.value;
          this.dispatchEvent(new CustomEvent("message", { detail: data }));
        });
        await this.msgChar.startNotifications();
      }

      // dispatch event
      this.dispatchEvent(new CustomEvent("connected"));
    });
  }

  async refresh() {
    await this.queue.run(async () => {
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
        await write(this.selectChar, name);
        this.cache[name] = await read(this.valueChar);
      }
    });
  }

  async unlock(password) {
    return await this.queue.run(async () => {
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

  async read(name) {
    return await this.queue.run(async () => {
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

  async write(name, value) {
    return await this.queue.run(async () => {
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

  async quickWrite(name, values, confirm = false) {
    return await this.queue.run(async () => {
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

  async flash(data, progress) {
    return await this.queue.run(async () => {
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

  async session(timeout) {
    // check characteristic
    if (!this.msgChar) {
      return;
    }

    // create queue
    const queue = new AsyncQueue();

    // prepare handler
    const handler = (event) => {
      queue.push(event.detail);
    };

    // subscribe to messages
    this.addEventListener("message", handler);

    return await Session.open(
      {
        write: async (msg) => {
          // write message
          await write(this.msgChar, msg, false);
        },
        read: async (timeout) => {
          // return next message
          return queue.pop(timeout);
        },
        close: () => {
          // unsubscribe from messages
          this.removeEventListener("message", handler);
        },
      },
      timeout
    );
  }

  async disconnect() {
    return await this.queue.run(async () => {
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
