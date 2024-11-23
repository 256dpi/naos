import { Session } from "./session";
import { pack } from "./utils";

export const UpdateEndpoint = 0x2;

export async function update(
  session: Session,
  data: Uint8Array,
  report: (count: number) => void = null,
  timeout: number = 30000
) {
  // send "begin" command
  let cmd = pack("oi", 0, data.length);
  await session.send(UpdateEndpoint, cmd, 0);

  // receive reply
  let [reply] = await session.receive(UpdateEndpoint, false, timeout);

  // verify reply
  if (reply.length !== 1 && reply[0] !== 0) {
    throw new Error("invalid message");
  }

  // TODO: Dynamically determine channel MTU.

  // write data in 500-byte chunks
  let num = 0;
  let offset = 0;
  while (offset < data.length) {
    // determine chunks size
    let chunkSize = Math.min(500, data.length - offset);
    let chunkData = data.slice(offset, offset + chunkSize);

    // determine acked
    let acked = num % 10 == 0;

    // send "write" command
    cmd = pack("oob", 1, acked ? 1 : 0, chunkData);
    await session.send(UpdateEndpoint, cmd, acked ? timeout : 0);

    // increment offset
    offset += chunkSize;

    // report offset
    if (report) {
      report(offset);
    }

    // increment counter
    num++;
  }

  // send "finish" command
  cmd = pack("o", 3);
  await session.send(UpdateEndpoint, cmd, 0);

  // receive reply
  [reply] = await session.receive(UpdateEndpoint, false, timeout);

  // verify reply
  if (reply.length !== 1 && reply[0] !== 1) {
    throw new Error("invalid message");
  }
}
