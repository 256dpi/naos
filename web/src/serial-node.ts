import { SerialPort } from "serialport";
import ReadlineParser from "@serialport/parser-readline";
import { Channel, Device, Message, Transport } from "./device";
import { concat, toBase64, toBuffer } from "./utils";

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

    // prepare flag
    let closed = false;

    // capture port ref for channel
    const port = this.port;
    const parser = this.port.pipe(new ReadlineParser({ delimiter: "\n" }));
    let onLine: ((line: string) => void) | null = null;
    let onPortClose: (() => void) | null = null;

    const transport: Transport = {
      start: (onData, onClose) => {
        onLine = (line: string) => {
          if (line.startsWith("NAOS!")) {
            const data = line.slice(5);
            const msg = Message.parse(
              Uint8Array.from(atob(data), (c) => c.charCodeAt(0))
            );
            if (msg) {
              onData(msg);
            }
          }
        };
        onPortClose = () => {
          closed = true;
          onClose();
        };

        parser.on("data", onLine);
        port.on("close", onPortClose);
      },
      write: async (msg: Message) => {
        const frame = concat(
          toBuffer("\nNAOS!"),
          toBase64(msg.build()),
          toBuffer("\n")
        );
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
        if (onLine) {
          parser.off("data", onLine);
        }
        if (onPortClose) {
          port.off("close", onPortClose);
        }
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

    this.ch = new Channel(transport, 1, () => {
      this.ch = null;
    });
    return this.ch;
  }
}
