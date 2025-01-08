import { Session } from "./session";
import { pack, toString } from "./utils";

const metricsEndpoint = 0x05;

export enum MetricKind {
  counter,
  gauge,
}

export enum MetricType {
  long,
  float,
  double,
}

export interface MetricInfo {
  ref: number;
  kind: MetricKind;
  type: MetricType;
  name: string;
  size: number;
}

export interface MetricLayout {
  keys: string[];
  values: string[][];
}

export async function listMetrics(
  s: Session,
  timeout: number = 5000
): Promise<MetricInfo[]> {
  // send command
  const cmd = pack("o", 0);
  await s.send(metricsEndpoint, cmd, 0);

  // prepare list
  const list: MetricInfo[] = [];

  for (;;) {
    // receive reply or return list on ack
    const [reply, ack] = await s.receive(metricsEndpoint, true, timeout);
    if (ack) {
      break;
    }

    // verify reply
    if (reply.length < 4) {
      throw new Error("Invalid reply");
    }

    // parse reply
    const ref = reply[0];
    const kind = reply[1];
    const type = reply[2];
    const size = reply[3];
    const name = toString(reply.slice(4));

    // append info
    list.push({ ref, kind, type, name, size });
  }

  return list;
}

export async function describeMetric(
  s: Session,
  ref: number,
  timeout: number = 5000
): Promise<MetricLayout> {
  // send command
  const cmd = pack("oo", 1, ref);
  await s.send(metricsEndpoint, cmd, 0);

  // prepare lists
  let keys: string[] = [];
  let values: string[][] = [];

  for (;;) {
    // receive reply
    const [reply, ack] = await s.receive(metricsEndpoint, true, timeout);
    if (ack) {
      break;
    }

    // verify reply
    if (reply.length < 1) {
      throw new Error("Invalid reply");
    }

    // handle key
    if (reply[0] === 0) {
      // verify reply
      if (reply.length < 3) {
        throw new Error("Invalid reply");
      }

      // parse reply
      const num = reply[1];
      const key = toString(reply.slice(2));

      // add key
      keys[num] = key;
      values[num] = [];

      continue;
    }

    // handle value
    if (reply[0] === 1) {
      // verify reply
      if (reply.length < 4) {
        throw new Error("Invalid reply");
      }

      // parse reply
      const numKey = reply[1];
      const numValue = reply[2];
      const value = toString(reply.slice(3));

      // add value
      values[numKey][numValue] = value;

      continue;
    }

    throw new Error("Invalid reply");
  }

  return {
    keys: keys,
    values: values,
  };
}

export async function readMetrics(
  s: Session,
  ref: number,
  timeout: number = 5000
): Promise<Uint8Array> {
  // send command
  const cmd = pack("oo", 2, ref);
  await s.send(metricsEndpoint, cmd, 0);

  // receive reply
  const [reply] = await s.receive(metricsEndpoint, false, timeout);

  return reply;
}

export async function readLongMetrics(
  s: Session,
  ref: number,
  timeout: number = 5000
): Promise<number[]> {
  // receive value
  const reply = await readMetrics(s, ref, timeout);

  // convert reply
  let list: number[] = [];
  for (let i = 0; i < reply.length; i += 4) {
    list.push(new DataView(reply.buffer).getInt32(i, true));
  }

  return list;
}

export async function readFloatMetrics(
  s: Session,
  ref: number,
  timeout: number = 5000
): Promise<number[]> {
  // receive value
  const reply = await readMetrics(s, ref, timeout);

  // convert reply
  let list: number[] = [];
  for (let i = 0; i < reply.length; i += 4) {
    list.push(new DataView(reply.buffer).getFloat32(i, true));
  }

  return list;
}

export async function readDoubleMetrics(
  s: Session,
  ref: number,
  timeout: number = 5000
): Promise<number[]> {
  // receive value
  const reply = await readMetrics(s, ref, timeout);

  // convert reply
  let list: number[] = [];
  for (let i = 0; i < reply.length; i += 8) {
    list.push(new DataView(reply.buffer).getFloat64(i, true));
  }

  return list;
}
