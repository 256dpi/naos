import { ManagedDevice, ManagedDeviceOptions, UUIDs } from "./device";

export class Manager {
  private device: ManagedDevice | null = null;

  async request(
    options: ManagedDeviceOptions = {
      subscribe: false,
      autoUpdate: false,
    }
  ): Promise<ManagedDevice | null> {
    // release existing device
    if (this.device) {
      if (this.device.connected) {
        this.device.disconnect().then();
      }
      this.device = null;
    }

    // request device
    let dev: BluetoothDevice | null;
    try {
      dev = await navigator.bluetooth.requestDevice({
        filters: [{ services: [UUIDs.service] }],
      });
    } catch (err) {
      // ignore
    }
    if (!dev) {
      return null;
    }

    // create device
    const device = new ManagedDevice(dev, options);

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
