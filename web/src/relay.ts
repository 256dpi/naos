import { Session } from "./session";
import { pack, unpack } from "./utils";
import { Channel, Device, Message, Transport } from "./device";
import { ManagedDevice } from "./managed";

const relayEndpoint = 0x04;

export async function relayCollect(
  s: Session,
  timeout: number = 5000
): Promise<number[]> {
  // send command
  const cmd = pack("o", 0);
  await s.send(relayEndpoint, cmd, 0);

  // receive reply or return list on ack
  const [reply] = await s.receive(relayEndpoint, false, timeout);

  // verify reply
  if (reply.length !== 8) {
    throw new Error("Invalid reply");
  }

  // unpack reply
  let raw = unpack("q", reply)[0] as bigint;

  // prepare map
  let list: number[] = [];
  for (let i = 0; i < 64; i++) {
    if ((raw & (BigInt(1) << BigInt(i))) != BigInt(0)) {
      list.push(i);
    }
  }

  return list;
}

export async function relayLink(
  s: Session,
  device: number,
  timeout: number = 5000
): Promise<void> {
  // send command
  const cmd = pack("oo", 1, device);
  await s.send(relayEndpoint, cmd, timeout);
}

export async function relaySend(
  s: Session,
  device: number,
  data: Uint8Array
): Promise<void> {
  // send command
  const cmd = pack("oob", 2, device, data);
  await s.send(relayEndpoint, cmd, 0);
}

export async function relayReceive(
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
    const host = this.host.device();
    if (!host) {
      throw new Error("device not available");
    }
    return `${host.id()}/${this.device}`;
  }

  type() {
    return "Relay";
  }

  name() {
    return `Relay: ${this.device}`;
  }

  async open(): Promise<Channel> {
    // check channel
    if (this.ch) {
      throw new Error("channel already open");
    }

    // open session
    const session = await this.host.newSession();

    // link device
    await relayLink(session, this.device);

    let closed = false;

    const transport: Transport = {
      start: (onData, onClose) => {
        (async () => {
          while (!closed) {
            try {
              const msg = Message.parse(await relayReceive(session));
              if (msg) {
                onData(msg);
              }
            } catch (e) {
              if (!closed) {
                console.error(e);
              }
              closed = true;
              onClose();
              break;
            }
          }
        })().catch(() => {});
      },
      write: async (msg: Message) => {
        await relaySend(session, this.device, msg.build());
      },
      close: async () => {
        closed = true;
        try {
          await session.end(1000);
        } catch (e) {
          // ignore
        }
      },
    };

    this.ch = new Channel(transport, this, 10, () => {
      this.ch = null;
    });
    return this.ch;
  }
}
