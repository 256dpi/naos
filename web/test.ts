import {
  bleRequest,
  collectParams,
  listDir,
  listParams,
  makeHTTPDevice,
  ManagedDevice,
  random,
  readFile,
  removePath,
  renamePath,
  requestFile,
  serialRequest,
  sha256File,
  statPath,
  toBuffer,
  toString,
  update,
  writeFile,
} from "./src";
import {
  describeMetric,
  listMetrics,
  MetricType,
  readDoubleMetrics,
  readFloatMetrics,
  readLongMetrics,
} from "./src/metrics";

let device: ManagedDevice | null = null;

async function ble() {
  // stop device
  if (device) {
    await device.stop();
    device = null;
  }

  // request device
  let dev = await bleRequest();
  if (!dev) {
    return;
  }

  console.log("Got Device:", dev);

  // create device
  device = new ManagedDevice(dev);

  // activate device
  await device.activate();

  // unlock locked device
  if (await device.locked()) {
    console.log("Unlock", await device.unlock(prompt("Password")));
  }

  console.log("Ready!");
}

async function serial() {
  // stop device
  if (device) {
    await device.stop();
    device = null;
  }

  // request device
  let dev = await serialRequest();
  if (!dev) {
    return;
  }

  console.log("Got Device:", dev);

  // create device
  device = new ManagedDevice(dev);

  // activate device
  await device.activate();

  // unlock locked device
  if (await device.locked()) {
    console.log("Unlock", await device.unlock(prompt("Password")));
  }

  console.log("Ready!");
}

async function http() {
  // stop device
  if (device) {
    await device.stop();
    device = null;
  }

  // request address
  const addr = prompt("Address", "192.168.1.1");

  // create device
  device = new ManagedDevice(makeHTTPDevice(addr));

  // activate device
  await device.activate();

  // unlock locked device
  if (await device.locked()) {
    console.log("Unlock", await device.unlock(prompt("Password")));
  }

  console.log("Ready!");
}

async function params() {
  console.log("Testing Params...");

  await device.activate();

  await device.useSession(async (session) => {
    const params = await listParams(session);
    console.log(params);

    const updates = await collectParams(
      session,
      params.map((p) => p.ref),
      BigInt(0)
    );
    console.log(updates);
  });

  await device.deactivate();

  console.log("Done!");
}

async function flash(input: HTMLInputElement) {
  if (!device || !input.files.length) {
    return;
  }

  console.log("Flashing...");

  const data = new Uint8Array(await requestFile(input.files[0]));

  await device.activate();
  const session = await device.newSession();

  await update(session, data, (progress) => {
    console.log((progress / data.length) * 100);
  });

  await session.end(1000);
  await device.deactivate();

  console.log("Done!");
}

async function fs() {
  console.log("Testing FS...");

  await device.activate();

  await device.useSession(async (session) => {
    console.log(await statPath(session, "/lol.txt"));
    console.log(await listDir(session, "/"));

    console.log(toString(await readFile(session, "/lol.txt")));

    await writeFile(session, "/test.txt", toBuffer(random(16)));
    console.log(toString(await readFile(session, "/test.txt")));
    await renamePath(session, "/test.txt", "/test2.txt");
    console.log(await sha256File(session, "/test2.txt"));
    await removePath(session, "/test2.txt");

    // await write(session, "/data.bin", new Uint8Array(4096));
    // console.log(await stat(session, "/data.bin"));
    // console.log(await read(session, "/data.bin"));
  });

  console.log("Done!");
}

async function metrics() {
  console.log("Testing Metrics...");

  await device.activate();

  await device.useSession(async (session) => {
    let metrics = await listMetrics(session);
    console.log(metrics);

    for (let metric of metrics) {
      console.log(await describeMetric(session, metric.ref));
    }

    for (let metric of metrics) {
      switch (metric.type) {
        case MetricType.long:
          console.log(await readLongMetrics(session, metric.ref));
          break;
        case MetricType.float:
          console.log(await readFloatMetrics(session, metric.ref));
          break;
        case MetricType.double:
          console.log(await readDoubleMetrics(session, metric.ref));
          break;
      }
    }
  });

  console.log("Done!");
}

window["_ble"] = ble;
window["_serial"] = serial;
window["_http"] = http;
window["_params"] = params;
window["_flash"] = flash;
window["_fs"] = fs;
window["_metrics"] = metrics;

// redirect to localhost from '0.0.0.0'
if (location.hostname === "0.0.0.0") {
  location.hostname = "localhost";
}
