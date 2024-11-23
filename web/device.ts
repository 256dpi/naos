import { AsyncQueue } from "./queue";

/**
 * Device represents a device that can be communicated with.
 */
export interface Device {
  /**
   * Returns a stable identifier for the device.
   */
  id(): string;

  /**
   * Open opens a channel to the device. An opened channel must fail or be
   * closed before another channel can be opened.
   */
  open(): Promise<Channel>;
}

/**
 * Queue is used to receive messages from a channel.
 */
export class Queue extends AsyncQueue<Uint8Array> {}

/**
 * QueueList is used to manage a list of queues.
 */
export class QueueList {
  private queues: Queue[] = [];

  /**
   * Adds a queue to the list.
   */
  add(queue: Queue) {
    if (!this.queues.includes(queue)) {
      this.queues.push(queue);
    }
  }

  /**
   * Removes a queue from the list.
   */
  drop(queue: Queue) {
    const index = this.queues.indexOf(queue);
    if (index >= 0) {
      this.queues.splice(index, 1);
    }
  }

  /**
   * Dispatches data to all queues.
   */
  dispatch(data: Uint8Array) {
    for (let queue of this.queues) {
      queue.push(data);
    }
  }
}

/**
 * Channel provides the mechanism to exchange messages between a device and a client.
 */
export interface Channel {
  name(): string;
  valid(): boolean;
  subscribe(queue: Queue): void;
  unsubscribe(queue: Queue): void;
  write(data: Uint8Array): Promise<void>;
  close(): void;
}

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
}

/**
 * Read reads a message from the queue.
 */
export async function read(queue: Queue, timeout: number): Promise<Message> {
  // read from queue
  const data = await queue.pop(timeout);
  if (!data) {
    throw new Error("timeout");
  }

  // check length and version
  if (data.length < 4 || data[0] !== 1) {
    throw new Error("invalid message");
  }

  // get view
  const view = new DataView(data.buffer);

  return new Message(
    view.getUint16(1, true),
    data[3],
    data.length > 4 ? data.slice(4) : null
  );
}

/**
 * Write writes a message to the channel.
 */
export async function write(ch: Channel, msg: Message): Promise<void> {
  // prepare data
  const data = new Uint8Array(4 + msg.size());
  const view = new DataView(data.buffer);
  view.setUint8(0, 1); // version
  view.setUint16(1, msg.session, true);
  view.setUint8(3, msg.endpoint);
  if (msg.data) {
    data.set(msg.data, 4);
  }

  // write data
  await ch.write(data);
}
