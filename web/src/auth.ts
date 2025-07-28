import { Session } from "./session";
import { compare, hmac256, pack } from "./utils";

const authEndpoint = 0x6;

export interface AuthData {
  uuid: Uint8Array;
  product: number;
  revision: number;
  batch: number;
  date: number;
}

export async function authStatus(
  s: Session,
  timeout: number = 5000
): Promise<boolean> {
  // send command
  await s.send(authEndpoint, pack("o", 0), 0);

  // await reply
  const [reply] = await s.receive(authEndpoint, false, timeout);

  // verify reply
  if (reply.length !== 1) {
    throw new Error("invalid reply");
  }

  return reply[0] === 1;
}

export async function authProvision(
  s: Session,
  key: Uint8Array,
  data: AuthData,
  timeout: number = 5000
): Promise<void> {
  // validate key
  if (key.length !== 32) {
    throw new Error("key must be exactly 32 bytes");
  }

  // send command
  const cmd = pack(
    "obobhhhib",
    1, // command
    key,
    1, // version
    data.uuid,
    data.product,
    data.revision,
    data.batch,
    data.date,
    new Uint8Array(5).fill(0)
  );
  await s.send(authEndpoint, cmd, timeout);
}

export async function authDescribe(
  s: Session,
  key: Uint8Array = undefined,
  timeout: number = 5000
): Promise<AuthData> {
  // send command
  await s.send(authEndpoint, pack("o", 2), 0);

  // receive reply
  const [reply] = await s.receive(authEndpoint, false, timeout);

  // verify reply
  if (reply.length < 32) {
    throw new Error("invalid reply");
  }

  // check version
  if (reply[0] !== 1) {
    throw new Error(`invalid version: ${reply[0]}`);
  }

  // parse reply
  const uuid = reply.slice(1, 17);
  const product = reply[17] | (reply[18] << 8);
  const revision = reply[19] | (reply[20] << 8);
  const batch = reply[21] | (reply[22] << 8);
  const date =
    reply[23] | (reply[24] << 8) | (reply[25] << 16) | (reply[26] << 24);
  const signature = reply.slice(27, 32);

  // verify signature
  const expectedSignature = await hmac256(key, reply.slice(0, 27));
  if (compare(expectedSignature, signature)) {
    throw new Error("invalid signature");
  }

  return {
    uuid,
    product,
    revision,
    batch,
    date,
  };
}

export async function authAttest(
  s: Session,
  challenge: Uint8Array,
  timeout: number = 5000
): Promise<Uint8Array> {
  // send command
  await s.send(authEndpoint, pack("ob", 3, challenge), 0);

  // await reply
  const [reply] = await s.receive(authEndpoint, false, timeout);

  // verify reply
  if (reply.length !== 32) {
    throw new Error("invalid reply");
  }

  return reply;
}
