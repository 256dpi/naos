import Manager from "./manager.js";

async function run() {
  const manager = new Manager();

  const device = await manager.request();
  if (!device) {
    return;
  }

  console.log("Listening...");
  device.addEventListener("updated", (event) => {
    console.log(event.detail);
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
