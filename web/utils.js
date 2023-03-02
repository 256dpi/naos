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
