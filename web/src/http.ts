import { Channel, Device, Message, Transport } from "./device";

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

    const transport: Transport = {
      start: (onData, onClose) => {
        socket.onmessage = async (msg) => {
          const frame = Message.parse(
            new Uint8Array(await msg.data.arrayBuffer())
          );
          if (frame) {
            onData(frame);
          }
        };
        socket.onclose = () => {
          onClose();
        };
        socket.onerror = () => {
          onClose();
        };
      },
      write: async (msg: Message) => {
        socket.send(msg.build());
      },
      close: async () => {
        socket.close();
      },
    };

    this.ch = new Channel(transport, 10, () => {
      this.ch = null;
    });
    return this.ch;
  }
}
