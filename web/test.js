import {
  Manager,
  FSEndpoint,
  randomString,
  readFile,
  toString,
  toBuffer,
} from "./index.js";

let device;

async function run() {
  const manager = new Manager();

  device = await manager.request({
    subscribe: false,
    autoUpdate: false,
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

    // console.log("Refreshing...");
    // await device.refresh();
    //
    // console.log("Unlocking...");
    // await device.unlock("secret");
    //
    // console.log("Refreshing...");
    // await device.refresh();

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

async function fs() {
  if (!device) {
    return;
  }

  console.log("Opening session...");

  const session = await device.session();
  await session.ping();
  console.log(await session.query(0x03));

  const fs = new FSEndpoint(session);

  console.log(await fs.stat("/lol.txt"));
  console.log(await fs.list("/"));

  console.log(toString(await fs.read("/lol.txt")));

  await fs.write("/test.txt", toBuffer(randomString(16)));
  console.log(toString(await fs.read("/test.txt")));
  await fs.rename("/test.txt", "/test2.txt");
  console.log(await fs.sha256("/test2.txt"));
  await fs.remove("/test2.txt");

  // await fs.write("/data.bin", new Uint8Array(4096));
  // console.log(await fs.stat("/data.bin"));
  // console.log(await fs.read("/data.bin"));

  await session.end();

  console.log("Session closed!");
}

window._run = run;
window._flash = flash;
window._fs = fs;
