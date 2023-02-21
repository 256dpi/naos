const utf8End = new TextEncoder();
const utf8Dec = new TextDecoder();

async function write(char, str) {
  await char.writeValueWithResponse(utf8End.encode(str));
}

async function read(char) {
  const value = await char.readValue();
  return utf8Dec.decode(value);
}

export const UUIDs = {
  service: "632fba1b-4861-4e4f-8103-ffee9d5033b5",
  lockChar: "f7a5fba4-4084-239b-684d-07d5902eb591",
  listChar: "ac2289d1-231b-b78b-df48-7d951a6ea665",
  selectChar: "cfc9706d-406f-ccbe-4240-f88d6ed4bacd",
  valueChar: "01ca5446-8ee1-7e99-2041-6884b01e71b3",
  updateChar: "87bffdcf-0704-22a2-9c4a-7a61bc8c1726",
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

export default class Device extends EventTarget {
  device;
  service;
  lockChar;
  listChar;
  selectChar;
  valueChar;
  updateChar;

  protected = false;
  locked = false;
  parameters;
  cache = {};

  constructor(device) {
    super();

    // set device
    this.device = device;
  }

  /* Connection */

  get name() {
    return this.device.name;
  }

  get connected() {
    return this.device.gatt.connected;
  }

  async connect() {
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

    // handle updates
    this.updateChar.addEventListener("characteristicvaluechanged", (event) => {
      // get name
      const name = utf8Dec.decode(event.target.value);

      // dispatch event
      this.dispatchEvent(new CustomEvent("updated", { detail: name }));
    });

    // subscribe to updates
    await this.updateChar.startNotifications();
  }

  async refresh() {
    // check state
    if (!this.connected) {
      throw new Error("not connected");
    }

    // TODO: Prevent other actions from interfering with refresh.

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
      await this.read(param.name);
    }
  }

  async unlock(password) {
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
  }

  async read(name) {
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
  }

  async write(name, value) {
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
  }

  async disconnect() {
    // lock again if protected
    if (this.protected) {
      this.locked = true;
    }

    // disconnect
    await this.device.gatt.disconnect();
  }
}
