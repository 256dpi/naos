import {
  Manager,
  Device,
  Session,
  FSEndpoint,
  random,
  readFile,
  toString,
  toBuffer,
} from "./index";

let device: Device | null = null;

async function run() {
  const manager = new Manager();

  device = await manager.request({
    subscribe: false,
    autoUpdate: false,
  });
  if (!device) {
    return;
  }

  device.addEventListener("changed", (event: CustomEvent) => {
    console.log("changed", event.detail);
  });

  device.addEventListener("updated", (event: CustomEvent) => {
    console.log("updated", event.detail);
  });

  device.addEventListener("connected", async () => {
    console.log("connected");
  });

  device.addEventListener("disconnected", () => {
    console.log("disconnected");
  });
}

async function params() {
  console.log("Refreshing...");
  await device.refresh();
  console.log(device.parameters);

  // console.log("Unlocking...");
  // await device.unlock("secret");
  //
  // console.log("Refreshing...");
  // await device.refresh();
}

async function flash(input) {
  if (!device || !input.files.length) {
    return;
  }

  const data = new Uint8Array(await readFile(input.files[0]));

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

  const channel = device.getChannel();
  const session = await Session.open(channel);
  await session.ping(1000);
  console.log(await session.query(0x03, 1000));

  const fs = new FSEndpoint(session);

  console.log(await fs.stat("/lol.txt"));
  console.log(await fs.list("/"));

  console.log(toString(await fs.read("/lol.txt")));

  await fs.write("/test.txt", toBuffer(random(16)));
  console.log(toString(await fs.read("/test.txt")));
  await fs.rename("/test.txt", "/test2.txt");
  console.log(await fs.sha256("/test2.txt"));
  await fs.remove("/test2.txt");

  // await fs.write("/data.bin", new Uint8Array(4096));
  // console.log(await fs.stat("/data.bin"));
  // console.log(await fs.read("/data.bin"));

  await session.end(1000);

  console.log("Session closed!");
}

window["_run"] = run;
window["_params"] = params;
window["_flash"] = flash;
window["_fs"] = fs;
