import { Channel, Device, Message, Transport } from "./device";
import { concat, fromBase64, toBase64, toBuffer, toString } from "./utils";

export async function serialRequest(baudRate = 115200): Promise<Device | null> {
  // request port
  let port: SerialPort | null;
  try {
    port = await navigator.serial.requestPort();
  } catch (err) {
    // ignore
  }
  if (!port) {
    return null;
  }

  return new SerialDevice(port, baudRate);
}

export class SerialDevice implements Device {
  private readonly port: SerialPort;
  private readonly baudRate: number;
  private ch: Channel | null = null;

  constructor(port: SerialPort, baudRate: number) {
    // store port and baud rate
    this.port = port;
    this.baudRate = baudRate;

    // close open channel if disconnected
    this.port.addEventListener("disconnect", () => {
      if (this.ch) {
        this.ch.close().catch(() => {});
        this.ch = null;
      }
    });
  }

  id() {
    const info = this.port.getInfo();
    return `serial/${info.usbProductId ?? "unknown"}`;
  }

  type() {
    return "Serial";
  }

  name() {
    const info = this.port.getInfo();
    return `${info.usbProductId ?? "unknown"}`;
  }

  async open(): Promise<Channel> {
    // check channel
    if (this.ch) {
      throw new Error("channel already open");
    }

    // connect, if not connected already
    if (!this.port.readable) {
      await this.port.open({
        baudRate: this.baudRate,
      });
    }

    // create reader
    const reader = this.port.readable.getReader();

    // read data
    const read = async (
      onData: (msg: Message) => void,
      onClose: () => void
    ) => {
      try {
        let buffer = "";

        while (true) {
          // read data
          const { done, value } = await reader.read();
          if (done) {
            break;
          }

          // Decode the chunk and add it to the buffer
          buffer += toString(value);

          // Split the buffer into lines
          let lines = buffer.split("\n");

          // Process all complete lines
          for (let i = 0; i < lines.length - 1; i++) {
            const line = lines[i].replace(/\r$/, "");
            if (line.startsWith("NAOS!")) {
              try {
                const msg = Message.parse(fromBase64(line.slice(5)));
                if (msg) {
                  onData(msg);
                }
              } catch (err) {
                console.error("Error decoding message:", err);
              }
            }
          }

          // Save the last incomplete line back to the buffer
          buffer = lines[lines.length - 1];
        }
      } catch (err) {
        console.error("Error reading stream:", err);
      } finally {
        onClose();
        reader.releaseLock();
      }
    };

    // create writer
    const writer = this.port.writable.getWriter();

    const transport: Transport = {
      start: (onData, onClose) => {
        read(onData, onClose).catch(() => {});
      },
      write: async (msg: Message) => {
        await writer.write(
          concat(toBuffer("\nNAOS!"), toBase64(msg.build()), toBuffer("\n"))
        );
      },
      close: async () => {
        await writer.close();
        writer.releaseLock();
        await reader.cancel();
        // lock released by reader
      },
    };

    this.ch = new Channel(transport, this, 1, () => {
      this.ch = null;
    });
    return this.ch;
  }
}
