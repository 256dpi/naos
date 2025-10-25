import { Channel, Device, Queue, QueueList } from "./device";

export function makeHTTPDevice(addr: string): Device {
  return new HTTPDevice(addr);
}

export class HTTPDevice implements Device {
  private readonly address: string;
  private ch: Channel | null = null;

  constructor(address: string) {
    // store address
    this.address = address;
  }

  id() {
    return "http/" + this.address;
  }

  async open(): Promise<Channel> {
    // check channel
    if (this.ch) {
      throw new Error("channel already open");
    }

    // create socket
    const socket = new WebSocket("ws://" + this.address);

    // await connections
    await new Promise((resolve, reject) => {
      socket.onopen = resolve;
      socket.onerror = reject;
    });

    // create list
    const subscribers = new QueueList();

    // handle messages
    socket.onmessage = async (msg) => {
      const data = new Uint8Array(await msg.data.arrayBuffer());
      subscribers.dispatch(data);
    };

    // create channel
    this.ch = {
      name: () => "http",
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
        socket.send(data);
      },
      close: async () => {
        socket.close();
        this.ch = null;
      },
    };

    return this.ch;
  }
}
