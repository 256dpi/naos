import { AsyncQueue } from "./queue";
import { toString, toView } from "./utils";

/**
 * Device represents a device that can be communicated with.
 */
export interface Device {
  /**
   * Returns a stable identifier for the device.
   */
  id(): string;

  /**
   * Returns the device transport type.
   */
  type(): string;

  /**
   * Returns a user-facing device name.
   */
  name(): string;

  /**
   * Open opens a channel to the device. An opened channel must fail or be
   * closed before another channel can be opened.
   */
  open(): Promise<Channel>;
}

/**
 * Transport exchanges raw messages with a device or peer.
 */
export interface Transport {
  start(
    onData: (msg: Message) => void,
    onClose: () => void
  ): void | Promise<void>;
  write(msg: Message): Promise<void>;
  close(): Promise<void>;
}

/**
 * Queue is used to receive messages from a channel.
 */
export class Queue extends AsyncQueue<Message> {}

/**
 * Message represents a message exchanged between a device and client.
 */
export class Message {
  session: number; // uint16
  endpoint: number; // uint8
  data: Uint8Array | null;

  constructor(session: number, endpoint: number, data: Uint8Array | null) {
    this.session = session;
    this.endpoint = endpoint;
    this.data = data;
  }

  /**
   * Returns the size of the message.
   */
  size(): number {
    return this.data?.length ?? 0;
  }

  static parse(data: Uint8Array): Message | null {
    if (data.length < 4 || data[0] !== 1) {
      return null;
    }

    const view = toView(data);

    return new Message(
      view.getUint16(1, true),
      data[3],
      data.length > 4 ? data.slice(4) : null
    );
  }

  build(): Uint8Array {
    const data = new Uint8Array(4 + this.size());
    const view = toView(data);
    view.setUint8(0, 1);
    view.setUint16(1, this.session, true);
    view.setUint8(3, this.endpoint);
    if (this.data) {
      data.set(this.data, 4);
    }
    return data;
  }
}

/**
 * Channel wraps a raw transport in a session-aware channel.
 */
export class Channel {
  private readonly tr: Transport;
  private readonly dev: Device | null;
  private readonly widthValue: number;
  private readonly onClose?: () => void;
  private closed = false;
  private readonly queues = new Set<Queue>();
  private readonly opening = new Map<string, Queue>();
  private readonly sessions = new Map<number, Queue>();
  private readonly closing = new Map<number, Queue>();

  constructor(
    tr: Transport,
    device: Device | null,
    width: number,
    onClose?: () => void
  ) {
    this.tr = tr;
    this.dev = device;
    this.widthValue = width;
    this.onClose = onClose;
    const start = this.tr.start(
      (msg) => {
        for (const queue of this.route(msg)) {
          queue.push(msg);
        }
      },
      () => {
        if (!this.closed) {
          void this.close();
        }
      }
    );
    Promise.resolve(start).catch(() => {
      if (!this.closed) {
        void this.close();
      }
    });
  }

  width(): number {
    return this.widthValue;
  }

  device(): Device | null {
    return this.dev;
  }

  subscribe(queue: Queue) {
    this.queues.add(queue);
  }

  unsubscribe(queue: Queue) {
    this.queues.delete(queue);

    for (const [handle, owner] of this.opening.entries()) {
      if (owner === queue) {
        this.opening.delete(handle);
      }
    }
    for (const [session, owner] of this.sessions.entries()) {
      if (owner === queue) {
        this.sessions.delete(session);
        this.closing.delete(session);
      }
    }
    for (const [session, owner] of this.closing.entries()) {
      if (owner === queue) {
        this.closing.delete(session);
      }
    }
  }

  async write(queue: Queue | null, msg: Message): Promise<void> {
    if (!queue) {
      await this.tr.write(msg);
      return;
    }

    if (msg.session !== 0) {
      const owner = this.sessions.get(msg.session);
      if (owner && owner !== queue) {
        throw new Error("wrong owner");
      }
    }

    if (msg.session === 0 && msg.endpoint === 0x0) {
      this.opening.set(msg.data ? toString(msg.data) : "", queue);
    }
    if (msg.session !== 0 && msg.endpoint === 0xff) {
      this.closing.set(msg.session, queue);
    }

    try {
      await this.tr.write(msg);
    } catch (err) {
      if (
        msg.session === 0 &&
        msg.endpoint === 0x0 &&
        this.opening.get(msg.data ? toString(msg.data) : "") === queue
      ) {
        this.opening.delete(msg.data ? toString(msg.data) : "");
      }
      if (
        msg.session !== 0 &&
        msg.endpoint === 0xff &&
        this.closing.get(msg.session) === queue
      ) {
        this.closing.delete(msg.session);
      }

      throw err;
    }
  }

  async close(): Promise<void> {
    if (this.closed) {
      return;
    }

    this.closed = true;
    try {
      await this.tr.close();
    } finally {
      this.onClose?.();
    }
  }

  private route(msg: Message): Queue[] {
    if (msg.endpoint === 0x0) {
      const owner = this.opening.get(msg.data ? toString(msg.data) : "");
      if (owner && this.queues.has(owner)) {
        this.opening.delete(msg.data ? toString(msg.data) : "");
        this.sessions.set(msg.session, owner);
        return [owner];
      }
    }

    if (msg.session !== 0) {
      const owner = this.sessions.get(msg.session);
      if (owner && this.queues.has(owner)) {
        if (msg.endpoint === 0xff && msg.size() === 0) {
          this.sessions.delete(msg.session);
          this.closing.delete(msg.session);
        }
        return [owner];
      }

      this.sessions.delete(msg.session);
      this.closing.delete(msg.session);
      return [];
    }

    return Array.from(this.queues);
  }
}

/**
 * Read reads a message from the queue.
 */
export async function read(queue: Queue, timeout: number): Promise<Message> {
  const msg = await queue.pop(timeout);
  if (!msg) {
    throw new Error("timeout");
  }

  return msg;
}
