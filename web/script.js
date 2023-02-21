import Device, { UUIDs } from "./device";

export async function run() {
  let dev = await navigator.bluetooth.requestDevice({
    filters: [{ services: [UUIDs.service] }],
  });

  let device = new Device(dev);

  await device.connect();
  await device.refresh();
  await device.unlock("secret");
  await device.refresh();

  console.log(device);

  device.addEventListener("updated", (event) => {
    console.log(event.detail);
  });

  // await device.disconnect();
}

window._run = run;
