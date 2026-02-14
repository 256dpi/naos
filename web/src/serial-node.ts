import { SerialPort } from "serialport";
import { ReadlineParser } from "@serialport/parser-readline";
import { Channel, Device, Queue, QueueList } from "./device";
import { toBase64, toBuffer } from "./utils";

const knownPrefixes = ["tty.SLAB", "tty.usbserial", "tty.usbmodem", "ttyUSB"];

export async function listSerialPorts(): Promise<string[]> {
  const ports = await SerialPort.list();

  // get paths, sort reverse to list combined ports with serial port first
  const paths = ports
    .map((p) => p.path)
    .sort()
    .reverse();

  // filter to known prefixes
  return paths.filter((path) =>
    knownPrefixes.some((prefix) => path.includes(prefix))
  );
}

export class NodeSerialDevice implements Device {
  private readonly path: string;
  private readonly baudRate: number;
  private port: SerialPort | null = null;
  private ch: Channel | null = null;

  constructor(path: string, baudRate = 115200) {
    this.path = path;
    this.baudRate = baudRate;
  }

  id() {
    return `serial/${this.path}`;
  }

  async open(): Promise<Channel> {
    // check channel
    if (this.ch) {
      throw new Error("channel already open");
    }

    // open port
    this.port = await new Promise<SerialPort>((resolve, reject) => {
      const port = new SerialPort(
        { path: this.path, baudRate: this.baudRate },
        (err) => {
          if (err) {
            reject(err);
          } else {
            resolve(port);
          }
        }
      );
    });

    // create list
    const subscribers = new QueueList();

    // parse lines and dispatch NAOS frames
    const parser = this.port.pipe(new ReadlineParser({ delimiter: "\n" }));
    parser.on("data", (line: string) => {
      if (line.startsWith("NAOS!")) {
        const data = line.slice(5);
        subscribers.dispatch(
          Uint8Array.from(atob(data), (c) => c.charCodeAt(0))
        );
      }
    });

    // prepare flag
    let closed = false;

    // handle close
    this.port.on("close", () => {
      closed = true;
      if (this.ch) {
        this.ch = null;
      }
    });

    // capture port ref for channel
    const port = this.port;

    // create channel
    this.ch = {
      name: () => "serial",
      valid: () => !closed,
      width: () => 1,
      subscribe: (q: Queue) => {
        subscribers.add(q);
      },
      unsubscribe: (queue: Queue) => {
        subscribers.drop(queue);
      },
      write: async (data: Uint8Array) => {
        const frame = new Uint8Array([
          ...toBuffer("NAOS!"),
          ...toBase64(data.buffer),
          ...toBuffer("\n"),
        ]);
        await new Promise<void>((resolve, reject) => {
          port.write(frame, (err) => {
            if (err) {
              reject(err);
            } else {
              port.drain((err) => (err ? reject(err) : resolve()));
            }
          });
        });
      },
      close: async () => {
        closed = true;
        this.ch = null;
        await new Promise<void>((resolve, reject) => {
          port.close((err) => {
            if (err) {
              reject(err);
            } else {
              resolve();
            }
          });
        });
      },
    };

    return this.ch;
  }
}
