import { pack, toString } from "./utils.js";

export class FSEndpoint {
  #session;
  #timeout;

  constructor(session, timeout = 5000) {
    this.#session = session;
    this.#timeout = timeout;
  }

  async stat(path) {
    // send command
    await this.send(pack("os", 0, path), false);

    // await reply
    const reply = await this.receive(false);

    // verify "info" reply
    if (reply.byteLength !== 6 || reply.getUint8(0) !== 1) {
      throw new Error("invalid message");
    }

    // parse "info" reply
    const isDir = reply.getUint8(1) === 1;
    const size = reply.getUint32(2, true);

    return {
      name: "",
      isDir: isDir,
      size: size,
    };
  }

  async list(path) {
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
      if (reply.byteLength < 7 || reply.getUint8(0) !== 1) {
        throw new Error("invalid message");
      }

      // parse "info" reply
      const isDir = reply.getUint8(1) === 1;
      const size = reply.getUint32(2, true);
      const name = toString(reply.buffer.slice(6));

      // add info
      infos.push({
        name: name,
        isDir: isDir,
        size: size,
      });
    }
  }

  async read(path, report) {
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
      let range = await this.readRange(path, offset, length, (pos) => {
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

  async readRange(path, offset, length, report) {
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
      if (reply.byteLength <= 5 || reply.getUint8(0) !== 2) {
        throw new Error("invalid message");
      }

      // get offset
      let replyOffset = reply.getUint32(1, true);

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

  async write(path, data, report) {
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

  async rename(from, to) {
    // prepare command
    let cmd = pack("osos", 6, from, 0, to);

    // send command
    await this.send(cmd, false);

    // await reply
    await this.receive(true);
  }

  async remove(path) {
    // prepare command
    let cmd = pack("os", 7, path);

    // send command
    await this.send(cmd, true);
  }

  async sha256(path) {
    // prepare command
    let cmd = pack("os", 8, path);

    // send command
    await this.send(cmd, false);

    // await reply
    let reply = await this.receive(false);

    // verify "chunk" reply
    if (reply.byteLength !== 33 || reply.getUint8(0) !== 3) {
      throw new Error("invalid message");
    }

    // get sum
    return new Uint8Array(reply.buffer.slice(1));
  }

  async end() {
    // end session
    await this.#session.end(this.#timeout);
  }

  // Helpers

  async receive(expectAck) {
    // receive reply
    let data = await this.#session.receive(0x3, expectAck, this.#timeout);
    if (!data) {
      return null;
    }

    // handle errors
    if (data.getUint8(0) === 0) {
      throw new Error("posix error: " + data.getUint8(1));
    }

    return data;
  }

  async send(data, awaitAck) {
    // send command
    await this.#session.send(0x3, data, awaitAck ? this.#timeout : 0);
  }
}
