import {
  bleRequest,
  collectParams,
  listDir,
  listParams,
  makeHTTPDevice,
  makePath,
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
  describeMetric,
  listMetrics,
  MetricType,
  readDoubleMetrics,
  readFloatMetrics,
  readLongMetrics,
  RelayDevice,
  scanRelay,
  authStatus,
  authProvision,
  authDescribe,
  authAttest,
  hmac256,
  compare,
  checkCoredump,
  readCoredump,
  deleteCoredump,
  streamLog,
  ParamType,
  ParamMode,
  MetricKind,
} from "./src";

// --- UI helpers ---

const logEl = document.getElementById("log")!;
const statusEl = document.getElementById("status")!;
const memoryEl = document.getElementById("memory-display")!;

function ts(): string {
  const d = new Date();
  return (
    String(d.getHours()).padStart(2, "0") +
    ":" +
    String(d.getMinutes()).padStart(2, "0") +
    ":" +
    String(d.getSeconds()).padStart(2, "0")
  );
}

type LogKind = "info" | "success" | "error" | "heading" | "data";

function log(msg: string, kind: LogKind = "info") {
  const el = document.createElement("div");
  el.className = `entry ${kind}`;
  const stamp = document.createElement("span");
  stamp.className = "timestamp";
  stamp.textContent = ts();
  el.appendChild(stamp);
  el.appendChild(document.createTextNode(msg));
  logEl.appendChild(el);
  logEl.scrollTop = logEl.scrollHeight;
}

function logData(label: string, value: unknown) {
  const text =
    typeof value === "object" ? JSON.stringify(value, null, 2) : String(value);
  log(`${label}: ${text}`, "data");
}

function check(condition: boolean, label: string) {
  if (condition) {
    log(`\u2713 ${label}`, "success");
  } else {
    log(`\u2717 ${label}`, "error");
  }
}

function setStatus(text: string, connected: boolean) {
  statusEl.textContent = text;
  statusEl.className = connected ? "connected" : "";
}

function clearLog() {
  logEl.innerHTML = "";
}

function addProgress(max: number): (value: number) => void {
  const bar = document.createElement("progress");
  bar.max = max;
  bar.value = 0;
  logEl.appendChild(bar);
  logEl.scrollTop = logEl.scrollHeight;
  return (value: number) => {
    bar.value = value;
    logEl.scrollTop = logEl.scrollHeight;
  };
}

// --- State ---

let device: ManagedDevice | null = null;

// --- Connection ---

async function ble() {
  if (device) {
    await device.stop();
    device = null;
  }

  let dev = await bleRequest();
  if (!dev) {
    return;
  }

  log("BLE device acquired", "success");

  device = new ManagedDevice(dev);
  await device.activate();

  if (await device.locked()) {
    const ok = await device.unlock(prompt("Password"));
    check(ok, "Unlock");
  }

  setStatus("BLE connected", true);
  log("Ready!", "success");
}

async function serial() {
  if (device) {
    await device.stop();
    device = null;
  }

  let dev = await serialRequest();
  if (!dev) {
    return;
  }

  log("Serial device acquired", "success");

  device = new ManagedDevice(dev);
  await device.activate();

  if (await device.locked()) {
    const ok = await device.unlock(prompt("Password"));
    check(ok, "Unlock");
  }

  setStatus("Serial connected", true);
  log("Ready!", "success");
}

async function http() {
  if (device) {
    await device.stop();
    device = null;
  }

  const addr = prompt("Address", "192.168.1.1");
  if (!addr) return;

  device = new ManagedDevice(makeHTTPDevice(addr));
  await device.activate();

  if (await device.locked()) {
    const ok = await device.unlock(prompt("Password"));
    check(ok, "Unlock");
  }

  setStatus(`HTTP ${addr}`, true);
  log("Ready!", "success");
}

// --- Tests ---

function paramTypeName(t: ParamType): string {
  return ParamType[t] || `unknown(${t})`;
}

function paramModeFlags(m: ParamMode): string {
  const flags: string[] = [];
  if (m & ParamMode.volatile) flags.push("volatile");
  if (m & ParamMode.system) flags.push("system");
  if (m & ParamMode.application) flags.push("application");
  if (m & ParamMode.locked) flags.push("locked");
  return flags.join(", ") || "none";
}

function decodeParamValue(value: Uint8Array): string {
  return toString(value);
}

async function params() {
  log("Params", "heading");

  await device.activate();

  await device.useSession(async (session) => {
    const params = await listParams(session);

    const updates = await collectParams(
      session,
      params.map((p) => p.ref),
      BigInt(0)
    );

    for (const p of params) {
      const u = updates.find((u) => u.ref === p.ref);
      const value = u ? `"${decodeParamValue(u.value)}"` : "n/a";
      logData(
        p.name,
        `${value}  [${paramTypeName(p.type)}, ${paramModeFlags(p.mode)}]`
      );
    }
  });

  await device.deactivate();
  log("Done!", "success");
}

async function flash(input: HTMLInputElement) {
  if (!device || !input.files.length) {
    return;
  }

  log("Flash", "heading");

  const data = new Uint8Array(await requestFile(input.files[0]));
  log(`Firmware size: ${data.length} bytes`);

  await device.activate();
  const session = await device.newSession();

  const setProgress = addProgress(100);
  await update(session, data, (progress) => {
    setProgress(Math.round((progress / data.length) * 100));
  });

  await session.end(1000);
  await device.deactivate();

  log("Done!", "success");
}

async function fs() {
  log("FS", "heading");

  await device.activate();

  await device.useSession(async (session) => {
    await makePath(session, "/TEST");
    const dirStat = await statPath(session, "/TEST");
    check(dirStat.isDir, "makePath");

    const content = random(64);
    await writeFile(session, "/TEST/TEST.TXT", toBuffer(content));
    check(true, "writeFile");

    const fileStat = await statPath(session, "/TEST/TEST.TXT");
    check(!fileStat.isDir, "statPath (is file)");
    check(
      fileStat.size === toBuffer(content).byteLength,
      "statPath (size match)"
    );

    const readBack = toString(await readFile(session, "/TEST/TEST.TXT"));
    check(readBack === content, "readFile (content match)");

    const entries = await listDir(session, "/TEST");
    check(entries.length === 1, "listDir (1 entry)");
    check(entries[0].name === "TEST.TXT", "listDir (name match)");

    const hash = await sha256File(session, "/TEST/TEST.TXT");
    check(hash.byteLength === 32, "sha256File (32 bytes)");

    await renamePath(session, "/TEST/TEST.TXT", "/TEST/RENAMED.TXT");
    const renamedStat = await statPath(session, "/TEST/RENAMED.TXT");
    check(renamedStat.size === fileStat.size, "renamePath (size preserved)");

    await removePath(session, "/TEST/RENAMED.TXT");
    await removePath(session, "/TEST");
    check(true, "removePath");
  });

  log("Done!", "success");
}

async function metrics() {
  log("Metrics", "heading");

  await device.activate();

  await device.useSession(async (session) => {
    const metrics = await listMetrics(session);

    for (const m of metrics) {
      // read layout and values
      const layout = await describeMetric(session, m.ref);
      let values: number[];
      switch (m.type) {
        case MetricType.long:
          values = await readLongMetrics(session, m.ref);
          break;
        case MetricType.float:
          values = await readFloatMetrics(session, m.ref);
          break;
        case MetricType.double:
          values = await readDoubleMetrics(session, m.ref);
          break;
      }

      // format header
      const kind = MetricKind[m.kind] || `kind(${m.kind})`;
      const type = MetricType[m.type] || `type(${m.type})`;
      log(`${m.name}  [${kind}, ${type}]`, "info");

      // format values with labels
      if (layout.keys.length > 0 && values.length > 0) {
        // build label combinations for each value slot
        let idx = 0;
        const formatValues = (keyIdx: number, prefix: string[]) => {
          if (keyIdx >= layout.keys.length) {
            const label = "- " + prefix.join(", ");
            const v = idx < values.length ? values[idx] : 0;
            logData(label, v);
            idx++;
            return;
          }
          for (const val of layout.values[keyIdx]) {
            formatValues(keyIdx + 1, [
              ...prefix,
              `${layout.keys[keyIdx]}=${val}`,
            ]);
          }
        };
        formatValues(0, []);
      } else {
        // flat values without labels
        for (const v of values) {
          logData("- value", v);
        }
      }
    }
  });

  log("Done!", "success");
}

async function relay() {
  log("Relay", "heading");

  await device.activate();

  await device.useSession(async (session) => {
    const devices = await scanRelay(session);
    log(
      `Found ${devices.length} relay device(s): ${devices.join(", ") || "none"}`
    );

    if (devices.length === 0) {
      return;
    }

    for (const id of devices) {
      log(`Device #${id}`, "info");

      const sub = new ManagedDevice(new RelayDevice(device, devices[0]));
      await sub.activate();

      await sub.useSession(async (subSession) => {
        const params = await listParams(subSession);
        const updates = await collectParams(
          subSession,
          params.map((p) => p.ref),
          BigInt(0)
        );
        for (const p of params) {
          const u = updates.find((u) => u.ref === p.ref);
          const value = u ? `"${decodeParamValue(u.value)}"` : "n/a";
          logData(
            `- ${p.name}`,
            `${value}  [${paramTypeName(p.type)}, ${paramModeFlags(p.mode)}]`
          );
        }
      });
    }
  });

  log("Done!", "success");
}

async function auth() {
  log("Auth", "heading");

  await device.activate();

  await device.useSession(async (session) => {
    let provisioned = await authStatus(session);
    log(provisioned ? "Device already provisioned" : "Device not provisioned");

    const key = toBuffer("0123456789abcdef0123456789abcdef");
    if (!provisioned) {
      await authProvision(session, key, {
        uuid: toBuffer("ABCDEF0123456789"),
        product: 1,
        revision: 2,
        batch: 3,
        date: Date.now() / 1000,
      });
      log("Provisioned device");
    }

    provisioned = await authStatus(session);
    if (!provisioned) {
      log("Failed to provision device", "error");
      return;
    }

    const data = await authDescribe(session, key);
    check(
      toString(data.uuid) === "ABCDEF0123456789" &&
        data.product === 1 &&
        data.revision === 2 &&
        data.batch === 3,
      "authDescribe"
    );

    const challenge = toBuffer(random(24));
    const result = await authAttest(session, challenge);
    const expected = await hmac256(key, challenge);
    check(
      result.length === 32 && compare(result, expected),
      "authAttest (HMAC match)"
    );

    log("Authentication successful!", "success");
  });
}

async function debug() {
  log("Debug", "heading");

  await device.activate();

  await device.useSession(async (session) => {
    const [size, reason] = await checkCoredump(session);
    logData("Coredump size", size);
    logData("Coredump reason", reason);

    if (size > 0) {
      const data = await readCoredump(session, 0, size);
      logData("Coredump data", `${data.length} bytes`);

      await deleteCoredump(session);
      log("Coredump deleted", "success");
    }

    log("Streaming log for 10s...");
    const ac = new AbortController();
    setTimeout(() => ac.abort(), 10000);
    await streamLog(session, ac.signal, (msg) => {
      log(msg, "data");
    });
    log("Log streaming stopped");
  });

  log("Done!", "success");
}

const ECHO_ENDPOINT = 0x08;

async function throughput() {
  log("Throughput", "heading");

  await device.activate();

  const session = await device.newSession();

  const mtu = await session.getMTU();
  logData("MTU", mtu);

  const payloadSize = mtu - 4;
  const payload = new Uint8Array(payloadSize);
  for (let i = 0; i < payloadSize; i++) {
    payload[i] = i & 0xff;
  }

  const rounds = 100;
  let totalBytes = 0;
  let errors = 0;

  const setProgress = addProgress(rounds);

  const start = performance.now();

  for (let i = 0; i < rounds; i++) {
    await session.send(ECHO_ENDPOINT, payload, 0);

    const [data] = await session.receive(ECHO_ENDPOINT, false, 5000);
    if (!data || data.length !== payload.length || !compare(payload, data)) {
      errors++;
      log(
        `Round ${i}: size mismatch (got ${data?.length}, expected ${payload.length})`,
        "error"
      );
      setProgress(i + 1);
      continue;
    }

    totalBytes += data.length;
    setProgress(i + 1);
  }

  const elapsed = performance.now() - start;
  const throughputKBs = totalBytes / 1024 / (elapsed / 1000);

  logData("Rounds", `${rounds}, Errors: ${errors}`);
  logData("Total", `${totalBytes} bytes in ${(elapsed / 1000).toFixed(2)}s`);
  logData("Throughput", `${throughputKBs.toFixed(2)} KB/s`);
  logData("Avg round-trip", `${(elapsed / rounds).toFixed(2)} ms`);

  await session.end(1000);

  log("Done!", "success");
}

async function memory() {
  log("Memory", "heading");

  await device.activate();

  const session = await device.newSession();
  const metrics = await listMetrics(session);
  const mem = metrics.find((m) => m.name === "free-memory");
  if (!mem) {
    await session.end(1000);
    log("Memory metric not found", "error");
    return;
  }

  const spinner = ["|", "/", "-", "\\"];
  let tick = 0;
  for (;;) {
    const values = await readLongMetrics(session, mem.ref);
    memoryEl.textContent =
      spinner[tick++ % 4] +
      " Free: " +
      values.map((v) => `${(v / 1024).toFixed(0)} KB`).join(" / ");
    await new Promise((r) => setTimeout(r, 1000));
  }
}

async function transfer() {
  log("Transfer", "heading");

  const kb = parseInt(prompt("Size in KB", "64"));
  if (!kb) return;
  const size = kb * 1024;
  const original = new Uint8Array(size);
  for (let i = 0; i < size; i++) {
    original[i] = i & 0xff;
  }
  logData("Generated", `${size} bytes`);

  await device.activate();

  await device.useSession(async (session) => {
    log("Uploading...");
    const uploadProgress = addProgress(size);
    const t1 = performance.now();
    await writeFile(session, "/transfer.bin", original, (count) => {
      uploadProgress(count);
    });
    const uploadTime = performance.now() - t1;
    logData(
      "Upload",
      `${size} bytes in ${(uploadTime / 1000).toFixed(2)}s (${(
        size /
        1024 /
        (uploadTime / 1000)
      ).toFixed(2)} KB/s)`
    );

    log("Downloading...");
    const downloadProgress = addProgress(size);
    const t2 = performance.now();
    const downloaded = await readFile(session, "/transfer.bin", (count) => {
      downloadProgress(count);
    });
    const downloadTime = performance.now() - t2;
    logData(
      "Download",
      `${downloaded.length} bytes in ${(downloadTime / 1000).toFixed(2)}s (${(
        downloaded.length /
        1024 /
        (downloadTime / 1000)
      ).toFixed(2)} KB/s)`
    );

    check(compare(original, downloaded), "Data integrity");

    await removePath(session, "/transfer.bin");
  });

  log("Done!", "success");
}

window["_ble"] = ble;
window["_serial"] = serial;
window["_http"] = http;
window["_params"] = params;
window["_flash"] = flash;
window["_fs"] = fs;
window["_metrics"] = metrics;
window["_relay"] = relay;
window["_auth"] = auth;
window["_debug"] = debug;
window["_throughput"] = throughput;
window["_transfer"] = transfer;
window["_memory"] = memory;
window["_clearLog"] = clearLog;

// redirect to localhost from '0.0.0.0'
if (location.hostname === "0.0.0.0") {
  location.hostname = "localhost";
}
