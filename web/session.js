import { pack, randomString, toBuffer, toString } from "./utils.js";

export function encodeMessage({ session, endpoint, data }) {
  // check data
  if (data && !(data instanceof Uint8Array)) {
    throw new Error("expected Uint8Array");
  }

  // pack message
  return pack("ohob", 1, session, endpoint, data || new Uint8Array(0));
}

export function decodeMessage(view) {
  // check message
  if (!ArrayBuffer.isView(view)) {
    throw new Error("expected DataView");
  }

  // check size
  if (view.byteLength < 4) {
    throw new Error("invalid message");
  }

  // check version
  if (view.getUint8(0) !== 1) {
    throw new Error("invalid version");
  }

  // parse session, endpoint and data
  const session = view.getUint16(1, true);
  const endpoint = view.getUint8(3);
  const data = new DataView(view.buffer.slice(4));

  return {
    session,
    endpoint,
    data,
  };
}

export class Session {
  #id = 0;
  #channel = null;

  static async open(channel, timeout) {
    // generate handle
    let outHandle = randomString(16);

    // send "begin" command
    await channel.write(
      encodeMessage({
        session: 0,
        endpoint: 0,
        data: toBuffer(outHandle),
      })
    );

    // wait for session ID reply
    let sid;
    while (true) {
      // TODO: Check overall timeout?

      // await message
      const reply = await channel.read(timeout);
      if (!reply) {
        return null;
      }

      // decode message
      const msg = decodeMessage(reply);

      // check endpoint and handle
      if (msg.endpoint !== 0 || toString(msg.data) !== outHandle) {
        continue;
      }

      // set session ID
      sid = msg.session;

      // create session
      return new Session(sid, channel);
    }
  }

  constructor(id, channel) {
    // set state
    this.#id = id;
    this.#channel = channel;
  }

  async ping(timeout = 5000) {
    // write command
    await this._write(0xfe);

    // read reply
    const msg = await this._read(timeout);

    // verify reply
    if (msg.endpoint !== 0xfe || msg.data.byteLength !== 1) {
      throw new Error("invalid message");
    } else if (msg.data.getUint8(0) !== 1) {
      throw new Error("session error: " + msg.data.getUint8(0));
    }
  }

  async query(endpoint, timeout = 5000) {
    // write command
    await this._write(endpoint);

    // read reply
    const msg = await this._read(timeout);

    // verify message
    if (msg.endpoint !== 0xfe || msg.data.byteLength !== 1) {
      throw new Error("invalid message");
    }

    return msg.data.getUint8(0) === 1;
  }

  async receive(endpoint, expectAck, timeout = 5000) {
    // await message
    const msg = await this._read(timeout);

    // handle acks
    if (msg.endpoint === 0xfe) {
      // check size
      if (msg.data.byteLength !== 1) {
        throw new Error("invalid message");
      }

      // check if OK
      if (msg.data.getUint8(0) === 1) {
        if (expectAck) {
          return null;
        } else {
          throw new Error("unexpected ack");
        }
      }

      throw new Error("session error: " + msg.data.getUint8(0));
    }

    // check endpoint
    if (msg.endpoint !== endpoint) {
      throw new Error("invalid message");
    }

    return msg.data;
  }

  async send(endpoint, data, ackTimeout = 5000) {
    // write message
    await this._write(endpoint, data);

    // return if timeout is zero
    if (ackTimeout === 0) {
      return;
    }

    // await reply
    const msg = await this._read(ackTimeout);

    // check reply
    if (msg.data.byteLength !== 1 || msg.endpoint !== 0xfe) {
      throw new Error("invalid message");
    } else if (msg.data.getUint8(0) !== 1) {
      throw new Error("session error: " + msg.data.getUint8(0));
    }
  }

  async end(timeout = 5000) {
    // write command
    await this._write(0xff);

    // read reply
    const msg = await this._read(timeout);

    // verify reply
    if (msg.endpoint !== 0xff || msg.data.byteLength > 0) {
      throw new Error("invalid message");
    }

    // close channel
    this.#channel.close();

    // clear state
    this.#id = 0;
    this.#channel = null;
  }

  // Helpers

  async _write(endpoint, data) {
    // forward message
    await this.#channel.write(
      encodeMessage({
        session: this.#id,
        endpoint: endpoint,
        data: data,
      })
    );
  }

  async _read(timeout) {
    while (true) {
      // TODO: Check overall timeout?

      // read next message
      let data = await this.#channel.read(timeout);
      if (!data) {
        return null;
      }

      // decode message
      const msg = decodeMessage(data);

      // check session ID
      if (msg.session !== this.#id) {
        continue;
      }

      return msg;
    }
  }
}
