import { Session } from "./session";
import { pack, unpack } from "./utils";
import { Channel, Device, Queue, QueueList } from "./device";
import { ManagedDevice } from "./managed";

const relayEndpoint = 0x04;

export async function scanRelay(
  s: Session,
  timeout: number = 5000
): Promise<number[]> {
  // send command
  const cmd = pack("o", 0);
  await s.send(relayEndpoint, cmd, 0);

  // receive reply or return list on ack
  const [reply] = await s.receive(relayEndpoint, false, timeout);

  // verify reply
  if (reply.length != 8) {
    throw new Error("Invalid reply");
  }

  // unpack reply
  let raw = unpack("q", reply)[0] as BigInt;

  // prepare map
  let list: number[] = [];
  for (let i = 0; i < 64; i++) {
    if ((raw & (BigInt(1) << BigInt(i))) != BigInt(0)) {
      list.push(i);
    }
  }

  return list;
}

export async function linkRelay(
  s: Session,
  device: number,
  timeout: number = 5000
): Promise<void> {
  // send command
  const cmd = pack("oo", 1, device);
  await s.send(relayEndpoint, cmd, timeout);
}

export async function sendRelay(
  s: Session,
  device: number,
  data: Uint8Array
): Promise<void> {
  // send command
  const cmd = pack("oob", 2, device, data);
  await s.send(relayEndpoint, cmd, 0);
}

export async function receiveRelay(
  s: Session,
  timeout: number = 5000
): Promise<Uint8Array> {
  // receive reply
  const [reply] = await s.receive(relayEndpoint, false, timeout);

  return reply;
}

export class RelayDevice implements Device {
  private host: ManagedDevice;
  private device: number;
  private ch: Channel | null = null;

  constructor(host: ManagedDevice, device: number) {
    // store host and device
    this.host = host;
    this.device = device;
  }

  id() {
    return `${this.host.device.id()}/${this.device}`;
  }

  async open(): Promise<Channel> {
    // check channel
    if (this.ch) {
      throw new Error("channel already open");
    }

    // open session
    const session = await this.host.newSession();

    // link device
    await linkRelay(session, this.device);

    // create list
    const subscribers = new QueueList();

    // run receiver
    (async () => {
      while (true) {
        try {
          // TODO: Use same trick as in swift to directly read from the session.
          const data = await receiveRelay(session);
          subscribers.dispatch(data);
        } catch (e) {
          console.error(e);
          break;
        }
      }
    })().then();

    // create channel
    this.ch = {
      name: () => "relay",
      valid() {
        return true;
      },
      subscribe: (q: Queue) => {
        subscribers.add(q);
      },
      unsubscribe(queue: Queue) {
        subscribers.drop(queue);
      },
      write: async (data: Uint8Array) => {
        await sendRelay(session, this.device, data);
      },
      close: async () => {
        await session.end(0);
        this.ch = null;
      },
    };

    return this.ch;
  }
}
