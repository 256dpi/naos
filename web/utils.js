const utf8Enc = new TextEncoder();
const utf8Dec = new TextDecoder();

export function toBuffer(string) {
  return utf8Enc.encode(string);
}

export function toString(buffer) {
  return utf8Dec.decode(buffer);
}

export function concat(buf1, buf2) {
  const buf = new Uint8Array(buf1.byteLength + buf2.byteLength);
  buf.set(new Uint8Array(buf1), 0);
  buf.set(new Uint8Array(buf2), buf1.byteLength);
  return buf;
}

export function readFile(file) {
  return new Promise((resolve, reject) => {
    const r = new FileReader();
    r.onload = () => {
      resolve(r.result);
    };
    r.onerror = (event) => {
      reject(event);
    };
    r.readAsArrayBuffer(file);
  });
}

export function pack(fmt /* ... */) {
  // get arguments
  const args = Array.from(arguments).slice(1);

  // calculate size
  let size = 0;
  for (const [index, arg] of args.entries()) {
    switch (fmt.charAt(index)) {
      case "s":
      case "b":
        size += arg.length;
        break;
      case "o":
        size += 1;
        break;
      case "h":
        size += 2;
        break;
      case "i":
        size += 4;
        break;
      case "q":
        size += 8;
        break;
      default:
        throw new Error("invalid format");
    }
  }

  // create buffer and view
  const buffer = new Uint8Array(size);
  const view = new DataView(buffer.buffer);

  // write arguments
  let offset = 0;
  for (const [index, arg] of args.entries()) {
    switch (fmt.charAt(index)) {
      case "s":
        buffer.set(toBuffer(arg), offset);
        offset += arg.length;
        break;
      case "b":
        buffer.set(arg, offset);
        offset += arg.length;
        break;
      case "o":
        view.setUint8(offset, arg);
        offset += 1;
        break;
      case "h":
        view.setUint16(offset, arg, true);
        offset += 2;
        break;
      case "i":
        view.setUint32(offset, arg, true);
        offset += 4;
        break;
      case "q":
        view.setBigUint64(offset, arg, true);
        offset += 8;
        break;
      default:
        throw new Error("invalid format");
    }
  }

  return buffer;
}

export function randomString(length) {
  const characters =
    "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789";
  let result = "";
  for (let i = 0; i < length; i++) {
    const randomIndex = Math.floor(Math.random() * characters.length);
    result += characters.charAt(randomIndex);
  }
  return result;
}

export class AsyncQueue {
  constructor() {
    this.queue = [];
    this.waiters = [];
  }

  push(item) {
    // add to back
    this.queue.push(item);

    // process queue
    while (this.waiters.length > 0 && this.queue.length > 0) {
      const resolve = this.waiters.shift();
      resolve(this.queue.shift());
    }
  }

  async pop(timeout) {
    // check if there is an item in the queue
    if (this.queue.length > 0) {
      return this.queue.shift();
    }

    return new Promise((resolve) => {
      // add waiter
      this.waiters.push(resolve);

      // handle timeout
      if (timeout > 0) {
        setTimeout(() => {
          if (this.waiters.includes(resolve)) {
            const index = this.waiters.indexOf(resolve);
            this.waiters.splice(index, 1);
            resolve(null);
          }
        }, timeout);
      }
    });
  }
}
