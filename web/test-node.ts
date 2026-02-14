import * as process from "node:process";
import { bluetooth } from "webbluetooth";

import {
  bleRequest,
  ManagedDevice,
  listParams,
  collectParams,
  listDir,
  statPath,
  readFile,
  writeFile,
  removePath,
  toString,
  toBuffer,
  random,
  listMetrics,
  describeMetric,
  readLongMetrics,
  readFloatMetrics,
  readDoubleMetrics,
  MetricType,
} from "./src";
import { listSerialPorts, NodeSerialDevice } from "./src/serial-node";

const tests: Record<string, (device: ManagedDevice) => Promise<void>> = {
  async params(device) {
    await device.useSession(async (session) => {
      const params = await listParams(session);
      console.log("Params:", params);

      const updates = await collectParams(
        session,
        params.map((p) => p.ref),
        BigInt(0)
      );
      console.log("Updates:", updates);
    });
  },

  async fs(device) {
    await device.useSession(async (session) => {
      console.log("Root:", await listDir(session, "/"));

      await writeFile(session, "/test.txt", toBuffer(random(16)));
      console.log("Written:", toString(await readFile(session, "/test.txt")));
      console.log("Stat:", await statPath(session, "/test.txt"));
      await removePath(session, "/test.txt");
      console.log("Removed /test.txt");
    });
  },

  async metrics(device) {
    await device.useSession(async (session) => {
      const metrics = await listMetrics(session);
      console.log("Metrics:", metrics);

      for (const metric of metrics) {
        console.log("Describe:", await describeMetric(session, metric.ref));
        switch (metric.type) {
          case MetricType.long:
            console.log("Value:", await readLongMetrics(session, metric.ref));
            break;
          case MetricType.float:
            console.log("Value:", await readFloatMetrics(session, metric.ref));
            break;
          case MetricType.double:
            console.log("Value:", await readDoubleMetrics(session, metric.ref));
            break;
        }
      }
    });
  },
};

async function main() {
  // parse arguments
  const args = process.argv.slice(2);

  // get transport
  const transport = args.shift();
  if (!transport || !["ble", "serial"].includes(transport)) {
    console.log(`Usage: npx tsx test-node.ts <ble|serial> <test...>`);
    console.log(`Available tests: ${Object.keys(tests).join(", ")}`);
    process.exit(1);
  }

  // get tests to run
  const selected = args;
  if (selected.length === 0) {
    console.log(`Usage: npx tsx test-node.ts <ble|serial> <test...>`);
    console.log(`Available tests: ${Object.keys(tests).join(", ")}`);
    process.exit(1);
  }

  // verify tests
  for (const name of selected) {
    if (!tests[name]) {
      console.error(`Unknown test: ${name}`);
      process.exit(1);
    }
  }

  // connect device
  let dev;
  if (transport === "ble") {
    console.log("Scanning for BLE device...");
    dev = await bleRequest(bluetooth);
    if (!dev) {
      console.error("No device found.");
      process.exit(1);
    }
  } else {
    const ports = await listSerialPorts();
    if (ports.length === 0) {
      console.error("No serial ports found.");
      process.exit(1);
    }
    console.log("Available ports:", ports.join(", "));
    const path = ports[0];
    console.log("Using:", path);
    dev = new NodeSerialDevice(path);
  }
  console.log("Found device:", dev.id());

  // create and activate managed device
  const device = new ManagedDevice(dev);
  await device.activate();
  console.log("Device activated.");

  // run selected tests
  try {
    for (const name of selected) {
      console.log(`\n--- ${name} ---`);
      await tests[name](device);
    }
  } finally {
    await device.stop();
  }

  console.log("\nDone!");
  process.exit(0);
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
