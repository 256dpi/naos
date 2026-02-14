import { Session } from "./session";
import { concat, pack, toString, unpack } from "./utils";

const debugEndpoint = 0x7;

export async function checkCoredump(
  s: Session,
  timeout: number = 5000
): Promise<[number, string]> {
  // send command
  const cmd = pack("o", 0);
  await s.send(debugEndpoint, cmd, 0);

  // receive reply
  const [reply] = await s.receive(debugEndpoint, false, timeout);

  // verify reply
  if (reply.length < 4) {
    throw new Error("invalid reply");
  }

  // parse size and reason
  const [size, reason] = unpack("is", reply);

  return [size, reason];
}

export async function readCoredump(
  s: Session,
  offset: number,
  length: number,
  timeout: number = 5000
): Promise<Uint8Array> {
  // send command
  const cmd = pack("oii", 1, offset, length);
  await s.send(debugEndpoint, cmd, 0);

  // prepare data
  let data = new Uint8Array(0);

  for (;;) {
    // receive reply or return data on ack
    const [reply, ack] = await s.receive(debugEndpoint, true, timeout);
    if (ack) {
      break;
    }

    // verify reply
    if (reply.length < 4) {
      throw new Error("invalid reply");
    }

    // get chunk offset
    const chunkOffset = unpack("i", reply)[0];

    // verify chunk offset
    if (chunkOffset !== offset + data.length) {
      throw new Error("invalid chunk offset");
    }

    // append chunk data
    data = concat(data, reply.slice(4));
  }

  return data;
}

export async function deleteCoredump(
  s: Session,
  timeout: number = 5000
): Promise<void> {
  // send command
  const cmd = pack("o", 2);
  await s.send(debugEndpoint, cmd, timeout);
}

export async function streamLog(
  s: Session,
  signal: AbortSignal,
  fn: (msg: string) => void
): Promise<void> {
  // start log, with checking ack
  await s.send(debugEndpoint, pack("o", 3), 5000);

  // mark last message
  let last = Date.now();

  for (;;) {
    // stop log if requested
    if (signal.aborted) {
      await s.send(debugEndpoint, pack("o", 4), 1000);
      return;
    }

    // receive log message
    try {
      const [data, ack] = await s.receive(debugEndpoint, true, 1000);

      // handle ack
      if (ack) {
        last = Date.now();
        continue;
      }

      // yield message
      last = Date.now();
      fn(toString(data));
    } catch (e) {
      // stop on any error except timeout
      if (e.message !== "timeout") {
        throw e;
      }

      // continue if a message was received recently
      if (Date.now() - last < 20000) {
        continue;
      }

      // otherwise, restart log without checking ack
      await s.send(debugEndpoint, pack("o", 3), 0);

      // update last message time
      last = Date.now();
    }
  }
}
