import Device, { UUIDs } from "./device";

export async function run() {
  let dev = await navigator.bluetooth.requestDevice({
    filters: [{ services: [UUIDs.service] }],
  });

  let device = new Device(dev);

  console.log("Connecting...");
  await device.connect();

  console.log("Refreshing...");
  await device.refresh();

  console.log("Unlocking...");
  await device.unlock("secret");

  console.log("Refreshing...");
  await device.refresh();

  console.log("Listening...");
  device.addEventListener("updated", (event) => {
    console.log(event.detail);
  });

  // await device.disconnect();
}

window._run = run;
