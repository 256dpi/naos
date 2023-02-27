import { Manager } from "./index.js";

async function run() {
  const manager = new Manager();

  const device = await manager.request({
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

  device.addEventListener("connected", async (event) => {
    console.log("Online!");

    console.log("Refreshing...");
    await device.refresh();

    console.log("Unlocking...");
    await device.unlock("secret");

    console.log("Refreshing...");
    await device.refresh();
  });

  device.addEventListener("disconnected", () => {
    console.log("Offline!");
  });
}

window._run = run;
