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
  return buf.buffer;
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
