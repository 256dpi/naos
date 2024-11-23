const utf8Enc = new TextEncoder();
const utf8Dec = new TextDecoder();

export function toBuffer(string: string): Uint8Array {
  return utf8Enc.encode(string);
}

export function toString(buffer: ArrayBufferLike): string {
  return utf8Dec.decode(buffer);
}

export function concat(buf1: Uint8Array, buf2: Uint8Array): Uint8Array {
  const buf = new Uint8Array(buf1.byteLength + buf2.byteLength);
  buf.set(buf1, 0);
  buf.set(buf2, buf1.byteLength);
  return buf;
}

export function random(length: number): string {
  const characters =
    "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789";
  let result = "";
  for (let i = 0; i < length; i++) {
    const randomIndex = Math.floor(Math.random() * characters.length);
    result += characters.charAt(randomIndex);
  }
  return result;
}

export function requestFile(file: File): Promise<ArrayBuffer> {
  return new Promise((resolve, reject) => {
    const r = new FileReader();
    r.onload = () => {
      resolve(r.result as ArrayBuffer);
    };
    r.onerror = (event) => {
      reject(event);
    };
    r.readAsArrayBuffer(file);
  });
}

export function pack(fmt: string, ...args: any[]): Uint8Array {
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

export function unpack(fmt: string, buffer: Uint8Array): any[] {
  // get view
  const view = new DataView(buffer.buffer);

  // prepare result
  const result: any[] = [];

  // read arguments
  let pos = 0;
  for (const code of fmt) {
    switch (code) {
      case "s": {
        let end = buffer.indexOf(0, pos);
        if (end === -1) end = buffer.length;
        result.push(toString(buffer.slice(pos, end)));
        pos = end;
        break;
      }
      case "b": {
        result.push(buffer.slice(pos));
        pos = buffer.length;
        break;
      }
      case "o": {
        result.push(buffer[pos]);
        pos += 1;
        break;
      }
      case "h": {
        result.push(view.getUint16(pos, true));
        pos += 2;
        break;
      }
      case "i": {
        result.push(view.getUint32(pos, true));
        pos += 4;
        break;
      }
      case "q": {
        result.push(view.getBigUint64(pos, true));
        pos += 8;
        break;
      }
      default:
        throw new Error(`Invalid format code: ${code}`);
    }
  }

  return result;
}
