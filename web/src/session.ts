import { Channel, Message, Queue, read, write } from "./device";
import { pack, random, toBuffer, toString, unpack } from "./utils";

export enum Status {
  locked = 1 << 0,
}

export class Session {
  private readonly id: number;
  private readonly ch: Channel;
  private readonly qu: Queue;
  private mtu: number = 0;

  static async open(ch: Channel): Promise<Session> {
    // prepare queue
    const queue = new Queue();

    // subscribe to channel
    ch.subscribe(queue);

    // prepare handle
    let handle = random(16);

    // begin session
    await write(ch, new Message(0, 0, toBuffer(handle)));

    // await reply
    let sid;
    for (;;) {
      const reply = await read(queue, 10 * 1000);
      if (reply.endpoint === 0 && toString(reply.data) === handle) {
        sid = reply.session;
        break;
      }
    }

    return new Session(sid, ch, queue);
  }

  constructor(id: number, ch: Channel, qu: Queue) {
    this.id = id;
    this.ch = ch;
    this.qu = qu;
  }

  async ping(timeout: number = 5000) {
    // write command
    await write(this.ch, new Message(this.id, 0xfe, null));

    // read reply
    const msg = await this.read(timeout);

    // verify reply
    if (msg.endpoint !== 0xfe || msg.size() !== 1) {
      throw new Error("invalid message");
    } else if (msg.data[0] !== 1) {
      throw new Error("session error: " + msg.data[0]);
    }
  }

  async query(endpoint: number, timeout: number = 5000) {
    // write command
    await write(this.ch, new Message(this.id, endpoint, null));

    // read reply
    const msg = await this.read(timeout);

    // verify message
    if (msg.endpoint !== 0xfe || msg.data.byteLength !== 1) {
      throw new Error("invalid message");
    }

    return msg.data[0] === 1;
  }

  async receive(
    endpoint: number,
    expectAck: boolean,
    timeout: number = 5000
  ): Promise<[Uint8Array | null, boolean]> {
    // await message
    const msg = await this.read(timeout);

    // handle ack
    if (msg.endpoint === 0xfe) {
      // check size
      if (msg.size() !== 1) {
        throw new Error("invalid ack size: " + msg.size());
      }

      // check if OK
      if (msg.data[0] === 1) {
        if (expectAck) {
          return [null, true];
        } else {
          throw new Error("unexpected ack");
        }
      }

      throw parseError(msg.data[0]);
    }

    // check endpoint
    if (msg.endpoint !== endpoint) {
      throw new Error("unexpected endpoint: " + msg.endpoint);
    }

    return [msg.data, false];
  }

  async send(endpoint: number, data: Uint8Array, ackTimeout: number) {
    // write message
    await write(this.ch, new Message(this.id, endpoint, data));

    // return if timeout is zero
    if (ackTimeout === 0) {
      return;
    }

    // await reply
    const msg = await this.read(ackTimeout);

    // check reply
    if (msg.data.byteLength !== 1 || msg.endpoint !== 0xfe) {
      throw new Error("invalid message");
    } else if (msg.data[0] !== 1) {
      throw parseError(msg.data[0]);
    }
  }

  async status(timeout: number = 5000): Promise<Status> {
    // write command
    let cmd = pack("o", 0);
    await this.send(0xfd, cmd, 0);

    // wait reply
    const [reply] = await this.receive(0xfd, false, timeout);

    // verify reply
    if (reply.length !== 1) {
      throw new Error("invalid message");
    }

    // unpack status
    let status = unpack("o", reply)[0];

    return status as Status;
  }

  async unlock(password: string, timeout: number = 5000): Promise<boolean> {
    // prepare command
    let cmd = pack("os", 1, password);
    await this.send(0xfd, cmd, 0);

    // wait reply
    const [reply] = await this.receive(0xfd, false, timeout);

    // verify reply
    if (reply.length !== 1) {
      throw new Error("invalid message");
    }

    return reply[0] === 1;
  }

  async getMTU(timeout: number = 5000): Promise<number> {
    // return cached value
    if (this.mtu > 0) {
      return this.mtu;
    }

    // write command
    let cmd = pack("o", 2);
    await this.send(0xfd, cmd, 0);

    // wait reply
    const [reply] = await this.receive(0xfd, false, timeout);

    // verify reply
    if (reply.length !== 2) {
      throw new Error("invalid message");
    }

    // cache value
    this.mtu = unpack("h", reply)[0];

    return this.mtu;
  }

  async end(timeout: number = 5000) {
    // write command
    await write(this.ch, new Message(this.id, 0xff, null));

    // read reply
    const msg = await this.read(timeout);

    // verify reply if available
    if (msg && (msg.endpoint !== 0xff || msg.size() > 0)) {
      throw new Error("invalid message");
    }

    // unsubscribe from channel
    this.ch.unsubscribe(this.qu);
  }

  async read(timeout: number): Promise<Message> {
    for (;;) {
      const msg = await read(this.qu, timeout);
      if (msg.session === this.id) {
        return msg;
      }
    }
  }
}

function parseError(num: number): Error {
  switch (num) {
    case 1:
      return new Error("invalid session");
    case 2:
      return new Error("invalid endpoint");
    case 3:
      return new Error("invalid data");
    default:
      return new Error("expected ack");
  }
}
