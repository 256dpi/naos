import { Session } from "./session";
import { pack, toString } from "./utils";

export const ParamsEndpoint = 0x01;

export enum ParamType {
  raw = 0,
  string,
  bool,
  long,
  double,
  action,
}

export enum ParamMode {
  volatile = 1 << 0,
  system = 1 << 1,
  application = 1 << 2,
  locked = 1 << 4,
}

export interface ParamInfo {
  ref: number;
  type: ParamType;
  mode: ParamMode;
  name: string;
}

export interface ParamUpdate {
  ref: number;
  age: bigint;
  value: Uint8Array;
}

export async function getParam(
  s: Session,
  name: string,
  timeout: number = 5000
): Promise<Uint8Array> {
  // prepare command
  const cmd = pack("os", 0, name);

  // send command
  await s.send(ParamsEndpoint, cmd, 0);

  // receive value
  const [data] = await s.receive(ParamsEndpoint, false, timeout);

  return data;
}

export async function setParam(
  s: Session,
  name: string,
  value: Uint8Array,
  timeout: number = 5000
): Promise<void> {
  // prepare command
  const cmd = pack("osob", 1, name, 0, value);

  // send command
  await s.send(ParamsEndpoint, cmd, timeout);
}

export async function listParams(
  s: Session,
  timeout: number = 5000
): Promise<ParamInfo[]> {
  // send command
  await s.send(ParamsEndpoint, pack("o", 2), 0);

  // prepare list
  const list: ParamInfo[] = [];

  for (;;) {
    // receive reply or return list on ack
    const [reply, ack] = await s.receive(ParamsEndpoint, true, timeout);
    if (ack) {
      break;
    }

    // verify reply
    if (reply.length < 4) {
      throw new Error("Invalid reply");
    }

    // parse reply
    const ref = reply[0];
    const type = reply[1];
    const mode = reply[2];
    const name = toString(reply.slice(3, -1));

    // TODO: Check type and mode.

    // append info
    list.push({ ref, type, mode, name });
  }

  return list;
}

export async function readParam(
  s: Session,
  ref: number,
  timeout: number = 5000
): Promise<Uint8Array> {
  // send command
  await s.send(ParamsEndpoint, pack("oo", 3, ref), 0);

  // receive value
  const [data] = await s.receive(ParamsEndpoint, false, timeout);

  return data;
}

export async function writeParam(
  s: Session,
  ref: number,
  value: Uint8Array,
  timeout: number = 5000
): Promise<void> {
  // prepare command
  const cmd = pack("oob", 4, ref, value);

  // send command
  await s.send(ParamsEndpoint, cmd, timeout);
}

export async function collectParams(
  s: Session,
  refs: number[],
  since: bigint,
  timeout: number = 5000
): Promise<ParamUpdate[]> {
  // prepare map
  let map: bigint = (BigInt(1) << BigInt(64)) - BigInt(1);
  if (refs.length > 0) {
    map = BigInt(0);
    for (const ref of refs) {
      map |= BigInt(1) << BigInt(ref);
    }
  }

  // prepare command
  const cmd = pack("oqq", 5, map, since);

  // send command
  await s.send(ParamsEndpoint, cmd, 0);

  // prepare list
  const list: ParamUpdate[] = [];

  for (;;) {
    // receive reply or return list on ack
    const [reply, ack] = await s.receive(ParamsEndpoint, false, timeout);
    if (ack) {
      break;
    }

    // verify reply
    if (reply.length < 9) {
      throw new Error("Invalid reply");
    }

    // parse reply
    const view = new DataView(reply.buffer);
    const ref = reply[0];
    const age = view.getBigUint64(1, true);
    const value = reply.slice(9, -1);

    // append info
    list.push({ ref, age, value });
  }

  return list;
}

export async function clearParam(
  s: Session,
  ref: number,
  timeout: number = 5000
): Promise<void> {
  // send command
  await s.send(ParamsEndpoint, pack("oo", 6, ref), timeout);
}
