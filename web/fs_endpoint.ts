import { Session } from "./session";
import { pack, toString } from "./utils";

export interface FSInfo {
  name: string;
  isDir: boolean;
  size: number;
}

export class FSEndpoint {
  private readonly session: Session;
  private readonly timeout: number;

  constructor(session: Session, timeout: number = 5000) {
    this.session = session;
    this.timeout = timeout;
  }

  async stat(path: string): Promise<FSInfo | null> {
    // send command
    await this.send(pack("os", 0, path), false);

    // await reply
    const reply = await this.receive(false);

    // verify "info" reply
    if (reply.length !== 6 || reply[0] !== 1) {
      throw new Error("invalid message");
    }

    // parse "info" reply
    const view = new DataView(reply.buffer);
    const isDir = reply[1] === 1;
    const size = view.getUint32(2, true);

    return {
      name: "",
      isDir: isDir,
      size: size,
    };
  }

  async list(path: string): Promise<FSInfo[]> {
    // send command
    await this.send(pack("os", 1, path), false);

    // prepare infos
    const infos = [];

    while (true) {
      // await reply
      const reply = await this.receive(true);
      if (!reply) {
        return infos;
      }

      // verify "info" reply
      if (reply.byteLength < 7 || reply[0] !== 1) {
        throw new Error("invalid message");
      }

      // parse "info" reply
      const view = new DataView(reply.buffer);
      const isDir = reply[1] === 1;
      const size = view.getUint32(2, true);
      const name = toString(reply.slice(6));

      // add info
      infos.push({
        name: name,
        isDir: isDir,
        size: size,
      });
    }
  }

  async read(
    path: string,
    report: (count: number) => void = null
  ): Promise<Uint8Array> {
    // stat file
    const info = await this.stat(path);

    // prepare data
    const data = new Uint8Array(info.size);

    // read file in chunks of 5 KB
    let offset = 0;
    while (offset < info.size) {
      // determine length
      const length = Math.min(5000, info.size - offset);

      // read range
      let range = await this.readRange(path, offset, length, (pos: number) => {
        if (report) {
          report(offset + pos);
        }
      });

      // append range
      data.set(range, offset);
      offset += range.byteLength;
    }

    return data;
  }

  async readRange(
    path: string,
    offset: number,
    length: number,
    report: (count: number) => void = null
  ): Promise<Uint8Array> {
    // send "open" command
    await this.send(pack("oos", 2, 0, path), true);

    // send "read" command
    await this.send(pack("oii", 3, offset, length), false);

    // prepare data
    let data = new Uint8Array(length);

    // prepare counter
    let count = 0;

    while (true) {
      // await reply
      let reply = await this.receive(true);
      if (!reply) {
        break;
      }

      // verify "chunk" reply
      if (reply.byteLength <= 5 || reply[0] !== 2) {
        throw new Error("invalid message");
      }

      // get offset
      let view = new DataView(reply.buffer);
      let replyOffset = view.getUint32(1, true);

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
    await this.send(pack("o", 5), true);

    return data;
  }

  async write(
    path: string,
    data: Uint8Array,
    report: (count: number) => void = null
  ) {
    // send "create" command (create & truncate)
    await this.send(pack("oos", 2, (1 << 0) | (1 << 2), path), true);

    // TODO: Dynamically determine channel MTU?

    // write data in 500-byte chunks
    let num = 0;
    let offset = 0;
    while (offset < data.byteLength) {
      // determine chunk size and chunk data
      let chunkSize = Math.min(500, data.byteLength - offset);
      let chunkData = data.slice(offset, offset + chunkSize);

      // determine mode
      let acked = num % 10 === 0;

      // prepare "write" command (acked or silent & sequential)
      let cmd = pack(
        "ooib",
        4,
        acked ? 0 : (1 << 0) | (1 << 1),
        offset,
        chunkData
      );

      // send "write" command
      await this.send(cmd, false);

      // receive ack or "error" replies
      if (acked) {
        await this.receive(true);
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
    await this.send(pack("o", 5), true);
  }

  async rename(from: string, to: string) {
    // prepare command
    let cmd = pack("osos", 6, from, 0, to);

    // send command
    await this.send(cmd, false);

    // await reply
    await this.receive(true);
  }

  async remove(path: string) {
    // prepare command
    let cmd = pack("os", 7, path);

    // send command
    await this.send(cmd, true);
  }

  async sha256(path: string) {
    // prepare command
    let cmd = pack("os", 8, path);

    // send command
    await this.send(cmd, false);

    // await reply
    let reply = await this.receive(false);

    // verify "chunk" reply
    if (reply.byteLength !== 33 || reply[0] !== 3) {
      throw new Error("invalid message");
    }

    // get sum
    return new Uint8Array(reply.buffer.slice(1));
  }

  // Helpers

  async receive(expectAck: boolean): Promise<Uint8Array> {
    // receive reply
    let [data] = await this.session.receive(0x3, expectAck, this.timeout);
    if (!data) {
      return null;
    }

    // handle errors
    if (data[0] === 0) {
      throw new Error("posix error: " + data[1]);
    }

    return data;
  }

  async send(data: Uint8Array, awaitAck: boolean) {
    // send command
    await this.session.send(0x3, data, awaitAck ? this.timeout : 0);
  }
}
