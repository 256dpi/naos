import { Session } from "./session";
import { pack, toView } from "./utils";

const timeEndpoint = 0x09;

/** Get the device's current wall-clock time in UTC at millisecond resolution. */
export async function getTime(
  s: Session,
  timeout: number = 5000
): Promise<Date> {
  // send command
  const cmd = pack("o", 0);
  await s.send(timeEndpoint, cmd, 0);

  // receive reply
  const [reply] = await s.receive(timeEndpoint, false, timeout);

  // verify reply
  if (reply.length !== 8) {
    throw new Error("invalid reply");
  }

  // parse epoch milliseconds
  const view = toView(reply);
  const ms = view.getBigInt64(0, true);

  return new Date(Number(ms));
}

/** Set the device's wall-clock time in UTC at millisecond resolution. */
export async function setTime(
  s: Session,
  date: Date,
  timeout: number = 5000
): Promise<void> {
  // build command
  const cmd = new Uint8Array(9);
  cmd[0] = 1;
  const view = toView(cmd);
  view.setBigInt64(1, BigInt(date.getTime()), true);

  // send command
  await s.send(timeEndpoint, cmd, timeout);
}

/** Get the device's current timezone offset from UTC in seconds. */
export async function getTimeInfo(
  s: Session,
  timeout: number = 5000
): Promise<number> {
  // send command
  const cmd = pack("o", 2);
  await s.send(timeEndpoint, cmd, 0);

  // receive reply
  const [reply] = await s.receive(timeEndpoint, false, timeout);

  // verify reply
  if (reply.length !== 4) {
    throw new Error("invalid reply");
  }

  // parse offset in seconds (signed)
  const view = toView(reply);
  return view.getInt32(0, true);
}
