import { Session } from "./session";
import { pack, unpack } from "./utils";

const fsEndpoint = 0x3;

export interface FSInfo {
  name: string;
  isDir: boolean;
  size: number;
}

export async function statPath(
  session: Session,
  path: string
): Promise<FSInfo | null> {
  // send command
  const cmd = pack("os", 0, path);
  await send(session, cmd, false);

  // await reply
  const reply = await receive(session, false);

  // verify "info" reply
  if (reply.length !== 6 || reply[0] !== 1) {
    throw new Error("invalid message");
  }

  // unpack "info" reply
  const args = unpack("oi", reply.slice(1));

  return {
    name: "",
    isDir: args[0] === 1,
    size: args[1],
  };
}

export async function listDir(
  session: Session,
  dir: string
): Promise<FSInfo[]> {
  // send command
  const cmd = pack("os", 1, dir);
  await send(session, cmd, false);

  // prepare infos
  const infos = [];

  while (true) {
    // await reply
    const reply = await receive(session, true);
    if (!reply) {
      return infos;
    }

    // verify "info" reply
    if (reply.byteLength < 7 || reply[0] !== 1) {
      throw new Error("invalid message");
    }

    // unpack "info" reply
    const args = unpack("ois", reply.slice(1));

    // add info
    infos.push({
      name: args[2],
      isDir: args[0] == 1,
      size: args[1],
    });
  }
}

export async function readFile(
  session: Session,
  file: string,
  report: (count: number) => void = null
): Promise<Uint8Array> {
  // stat file
  const info = await statPath(session, file);

  // prepare data
  const data = new Uint8Array(info.size);

  // read file in chunks of 5 KB
  let offset = 0;
  while (offset < info.size) {
    // determine length
    const length = Math.min(5000, info.size - offset);

    // read range
    let range = await readFileRange(
      session,
      file,
      offset,
      length,
      (pos: number) => {
        if (report) {
          report(offset + pos);
        }
      }
    );

    // append range
    data.set(range, offset);
    offset += range.byteLength;
  }

  return data;
}

export async function readFileRange(
  session: Session,
  file: string,
  offset: number,
  length: number,
  report: (count: number) => void = null
): Promise<Uint8Array> {
  // send "open" command
  let cmd = pack("oos", 2, 0, file);
  await send(session, cmd, true);

  // send "read" command
  cmd = pack("oii", 3, offset, length);
  await send(session, cmd, false);

  // prepare data
  let data = new Uint8Array(length);

  // prepare counter
  let count = 0;

  while (true) {
    // await reply
    let reply = await receive(session, true);
    if (!reply) {
      break;
    }

    // verify "chunk" reply
    if (reply.byteLength <= 5 || reply[0] !== 2) {
      throw new Error("invalid message");
    }

    // get offset
    let replyOffset = unpack("i", reply.slice(1))[0];

    // verify offset
    if (replyOffset !== offset + count) {
      throw new Error("invalid message");
    }

    // append data
    data.set(new Uint8Array(reply.buffer.slice(5)), count);

    // increment
    count += reply.byteLength - 5;

    // report length
    if (report) {
      report(count);
    }
  }

  // send "close" command
  cmd = pack("o", 5);
  await send(session, cmd, true);

  return data;
}

export async function writeFile(
  session: Session,
  file: string,
  data: Uint8Array,
  report: (count: number) => void = null
) {
  // send "create" command (create & truncate)
  let cmd = pack("oos", 2, (1 << 0) | (1 << 2), file);
  await send(session, cmd, true);

  // get MTU
  let mtu = await session.getMTU();

  // subtract overhead
  mtu -= 6;

  // write data in chunks
  let num = 0;
  let offset = 0;
  while (offset < data.byteLength) {
    // determine chunk size and chunk data
    let chunkSize = Math.min(mtu, data.byteLength - offset);
    let chunkData = data.slice(offset, offset + chunkSize);

    // determine mode
    let acked = num % 10 === 0;

    // prepare "write" command (acked or silent & sequential)
    cmd = pack("ooib", 4, acked ? 0 : (1 << 0) | (1 << 1), offset, chunkData);

    // send "write" command
    await send(session, cmd, false);

    // receive ack or "error" replies
    if (acked) {
      await receive(session, true);
    }

    // increment offset
    offset += chunkSize;

    // report offset
    if (report) {
      report(offset);
    }

    // increment count
    num += 1;
  }

  // send "close" command
  cmd = pack("o", 5);
  await send(session, cmd, true);
}

export async function renamePath(session: Session, from: string, to: string) {
  // send command
  let cmd = pack("osos", 6, from, 0, to);
  await send(session, cmd, false);

  // await reply
  await receive(session, true);
}

export async function removePath(session: Session, path: string) {
  // send command
  let cmd = pack("os", 7, path);
  await send(session, cmd, true);
}

export async function sha256File(session: Session, file: string) {
  // send command
  let cmd = pack("os", 8, file);
  await send(session, cmd, false);

  // await reply
  let reply = await receive(session, false);

  // verify "chunk" reply
  if (reply.byteLength !== 33 || reply[0] !== 3) {
    throw new Error("invalid message");
  }

  // return hash
  return new Uint8Array(reply.buffer.slice(1));
}

export async function makePath(session: Session, path: string) {
  // send command
  let cmd = pack("os", 9, path);
  await send(session, cmd, true);
}

/* Helpers */

async function receive(
  session: Session,
  expectAck: boolean,
  timeout = 5000
): Promise<Uint8Array> {
  // receive reply
  let [data] = await session.receive(fsEndpoint, expectAck, timeout);
  if (!data) {
    return null;
  }

  // handle errors
  if (data[0] === 0) {
    throw new Error("posix error: " + data[1]);
  }

  return data;
}

async function send(
  session: Session,
  data: Uint8Array,
  awaitAck: boolean,
  timeout = 5000
) {
  // send command
  await session.send(fsEndpoint, data, awaitAck ? timeout : 0);
}
