import {
  Manager,
  ManagedDevice,
  Session,
  random,
  toString,
  toBuffer,
  requestFile,
} from "./index";

import {
  listPath,
  readFile,
  renamePath,
  removePath,
  sha256File,
  statPath,
  writeFile,
} from "./fs";

let device: ManagedDevice | null = null;

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

  const data = new Uint8Array(await requestFile(input.files[0]));

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

  console.log(await statPath(session, "/lol.txt"));
  console.log(await listPath(session, "/"));

  console.log(toString(await readFile(session, "/lol.txt")));

  await writeFile(session, "/test.txt", toBuffer(random(16)));
  console.log(toString(await readFile(session, "/test.txt")));
  await renamePath(session, "/test.txt", "/test2.txt");
  console.log(await sha256File(session, "/test2.txt"));
  await removePath(session, "/test2.txt");

  // await write(session, "/data.bin", new Uint8Array(4096));
  // console.log(await stat(session, "/data.bin"));
  // console.log(await read(session, "/data.bin"));

  await session.end(1000);

  console.log("Session closed!");
}

window["_run"] = run;
window["_params"] = params;
window["_flash"] = flash;
window["_fs"] = fs;
