import { Channel, Device, Message, Transport } from "./device";

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

  type() {
    return "HTTP";
  }

  name() {
    return "Unnamed";
  }

  async open(): Promise<Channel> {
    // check channel
    if (this.ch) {
      throw new Error("channel already open");
    }

    // create socket
    const socket = new WebSocket("ws://" + this.address, "naos");

    // await connection
    await new Promise<void>((resolve, reject) => {
      socket.onopen = () => resolve();
      socket.onerror = () => {
        // close the half-open socket before failing, otherwise it lingers
        socket.close();
        reject(new Error("failed to open socket"));
      };
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

    this.ch = new Channel(transport, this, 10, () => {
      this.ch = null;
    });
    return this.ch;
  }
}
