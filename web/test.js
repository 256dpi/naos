import { Manager, readFile } from "./index.js";

let device;

async function run() {
  const manager = new Manager();

  device = await manager.request({
    subscribe: true,
    autoUpdate: true,
  });
  if (!device) {
    return;
  }

  console.log("Listening...");
  device.addEventListener("changed", (event) => {
    console.log("changed", event.detail);
  });
  device.addEventListener("updated", (event) => {
    console.log("updated", event.detail);
  });

  device.addEventListener("connected", async () => {
    console.log("Online!");

    console.log("Refreshing...");
    await device.refresh();

    console.log("Unlocking...");
    await device.unlock("secret");

    console.log("Refreshing...");
    await device.refresh();

    console.log("Ready!");
  });

  device.addEventListener("disconnected", () => {
    console.log("Offline!");
  });
}

async function flash(input) {
  if (!device || !input.files.length) {
    return;
  }

  const data = await readFile(input.files[0]);

  console.log("Flashing...", data);
  await device.flash(data, (progress) => {
    console.log(progress);
  });

  console.log("Flashing done!");
}

window._run = run;
window._flash = flash;
