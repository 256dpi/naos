import { Channel, Device, Message, Transport } from "./device";

const version = 0x1;
const cmdMsg = 0x0;

/**
 * ConnectDevice represents a device reachable through a NAOS Connect server.
 * The token is appended as a query parameter, as browser WebSockets cannot
 * set headers.
 */
export class ConnectDevice implements Device {
  private readonly url: string;
  private readonly token: string;
  private ch: Channel | null = null;

  constructor(url: string, token: string = "") {
    // store URL and token
    this.url = url;
    this.token = token;
  }

  id() {
    return "connect/" + this.url.replace(/^wss?:\/\//, "");
  }

  type() {
    return "Connect";
  }

  name() {
    return "Unnamed";
  }

  async open(): Promise<Channel> {
    // check channel
    if (this.ch) {
      throw new Error("channel already open");
    }

    // append token as query parameter
    let url = this.url;
    if (this.token) {
      url +=
        (url.includes("?") ? "&" : "?") +
        "token=" +
        encodeURIComponent(this.token);
    }

    // create socket
    const socket = new WebSocket(url, "naos");
    socket.binaryType = "arraybuffer";

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
        socket.onmessage = (event) => {
          // check frame header
          const frame = new Uint8Array(event.data as ArrayBuffer);
          if (frame.length < 2 || frame[0] !== version || frame[1] !== cmdMsg) {
            socket.close();
            return;
          }

          // parse message
          const msg = Message.parse(frame.slice(2));
          if (msg) {
            onData(msg);
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
        // frame message
        const data = msg.build();
        const frame = new Uint8Array(2 + data.length);
        frame[0] = version;
        frame[1] = cmdMsg;
        frame.set(data, 2);

        // write frame
        socket.send(frame as BufferSource);
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

/**
 * ConnectDescription describes a device listed by a NAOS Connect server.
 */
export interface ConnectDescription {
  uuid: string;
  connected: string;
  device_id?: string;
  device_name?: string;
  app_name?: string;
  app_version?: string;
  attach_url: string;
  attach_token?: string;
}

/**
 * Fetches the currently connected devices from a NAOS Connect server.
 */
export async function connectList(
  url: string,
  token: string = "",
): Promise<ConnectDescription[]> {
  // prepare headers
  const headers: Record<string, string> = {};
  if (token) {
    headers["Authorization"] = "Bearer " + token;
  }

  // perform request
  const res = await fetch(url, { headers });
  if (!res.ok) {
    throw new Error("unexpected status: " + res.status);
  }

  // parse response
  const out = await res.json();

  return out.devices ?? [];
}
