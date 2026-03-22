import { Channel, Device, Queue, QueueList } from "./device";
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

    // close open chanel if disconnected
    this.port.addEventListener("disconnect", () => {
      if (this.ch) {
        this.ch.close();
        this.ch = null;
      }
    });
  }

  id() {
    const info = this.port.getInfo();
    return `serial/${info.usbProductId ?? "unknown"}`;
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

    // create list
    const subscribers = new QueueList();

    // create reader
    const reader = this.port.readable.getReader();

    // track reader state
    let alive = true;

    // read data
    const read = async () => {
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
                subscribers.dispatch(fromBase64(line.slice(5)));
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
        alive = false;
        reader.releaseLock();
      }
    };

    // start reading
    read().then();

    // create writer
    const writer = this.port.writable.getWriter();

    // create channel
    this.ch = {
      name: () => "serial",
      valid() {
        return alive;
      },
      width() {
        return 1;
      },
      subscribe: (q: Queue) => {
        subscribers.add(q);
      },
      unsubscribe(queue: Queue) {
        subscribers.drop(queue);
      },
      write: async (data: Uint8Array) => {
        await writer.write(
          concat(toBuffer("\nNAOS!"), toBase64(data), toBuffer("\n"))
        );
      },
      close: async () => {
        await writer.close();
        writer.releaseLock();
        await reader.cancel();
        // lock released by reader
        this.ch = null;
      },
    };

    return this.ch;
  }
}
