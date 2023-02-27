import { Device, UUIDs } from "./device.js";

export class Manager extends EventTarget {
  device;

  async request(options = {}) {
    // release existing device
    if (this.device) {
      if (this.device.connected) {
        this.device.disconnect();
      }
      this.device = null;
    }

    // request device
    let device;
    try {
      device = await navigator.bluetooth.requestDevice({
        filters: [{ services: [UUIDs.service] }],
      });
    } catch (err) {
      // ignore
    }
    if (!device) {
      return;
    }

    // create device
    device = new Device(device, options);

    // set device
    this.device = device;

    // handle disconnected
    this.device.addEventListener("disconnected", async () => {
      // ignore if device changed
      if (this.device !== device) {
        return;
      }

      // otherwise re-connect
      await this.device.connect();
    });

    // initial connect
    setTimeout(async () => {
      await this.device.connect();
    }, 0);

    return this.device;
  }
}
