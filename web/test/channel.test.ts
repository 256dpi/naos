import * as assert from "node:assert/strict";
import { test } from "node:test";

import { Channel, Message, Queue, read, Transport } from "../src/device";
import { toBuffer } from "../src/utils";

class MockTransport implements Transport {
  writes: Message[] = [];
  closes = 0;
  private onData: ((msg: Message) => void) | null = null;
  private onClose: (() => void) | null = null;
  private onWrite: ((msg: Message) => void | Promise<void>) | null = null;

  start(onData: (msg: Message) => void, onClose: () => void) {
    this.onData = onData;
    this.onClose = onClose;
  }

  async write(msg: Message): Promise<void> {
    this.writes.push(msg);
    await this.onWrite?.(msg);
  }

  async close(): Promise<void> {
    this.closes += 1;
  }

  push(msg: Message) {
    assert.ok(this.onData, "transport not started");
    this.onData(msg);
  }

  fail() {
    this.onClose?.();
  }

  setOnWrite(fn: (msg: Message) => void | Promise<void>) {
    this.onWrite = fn;
  }
}

test("routes owned session traffic", async () => {
  const transport = new MockTransport();
  const channel = new Channel(transport, 10);
  const queue = new Queue();
  channel.subscribe(queue);

  const handle = toBuffer("open-owned");
  await channel.write(queue, new Message(0, 0x0, handle));

  transport.push(new Message(21, 0x0, handle));
  const openReply = await read(queue, 100);
  assert.equal(openReply.session, 21);

  transport.push(new Message(21, 0x42, toBuffer("payload")));
  const payload = await read(queue, 100);
  assert.equal(payload.endpoint, 0x42);
  assert.deepEqual(payload.data, toBuffer("payload"));

  transport.push(new Message(21, 0xff, new Uint8Array()));
  const close = await read(queue, 100);
  assert.equal(close.endpoint, 0xff);

  transport.push(new Message(21, 0x42, toBuffer("later")));
  await assert.rejects(read(queue, 25), /timeout/);
});

test("rejects wrong-owner writes", async () => {
  const transport = new MockTransport();
  const channel = new Channel(transport, 10);
  const owner = new Queue();
  const other = new Queue();
  channel.subscribe(owner);
  channel.subscribe(other);

  const handle = toBuffer("open-owner");
  await channel.write(owner, new Message(0, 0x0, handle));
  transport.push(new Message(9, 0x0, handle));
  await read(owner, 100);

  await assert.rejects(
    channel.write(other, new Message(9, 0x42, toBuffer("payload"))),
    /wrong owner/
  );
});

test("registers pending open before write returns", async () => {
  const transport = new MockTransport();
  const channel = new Channel(transport, 10);
  const queue = new Queue();
  channel.subscribe(queue);

  const handle = toBuffer("open-race");
  transport.setOnWrite(() => {
    transport.push(new Message(7, 0x0, handle));
    transport.push(new Message(7, 0x42, toBuffer("payload")));
  });

  await channel.write(queue, new Message(0, 0x0, handle));

  const openReply = await read(queue, 100);
  const payload = await read(queue, 100);
  assert.equal(openReply.session, 7);
  assert.equal(payload.endpoint, 0x42);
});

test("transport close closes channel once", async () => {
  const transport = new MockTransport();
  let closes = 0;
  const channel = new Channel(transport, 10, () => {
    closes += 1;
  });

  transport.fail();
  await new Promise((resolve) => setTimeout(resolve, 0));

  assert.equal(transport.closes, 1);
  assert.equal(closes, 1);

  await channel.close();
  assert.equal(transport.closes, 1);
  assert.equal(closes, 1);
});
