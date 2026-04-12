import { Queue } from "async-await-queue";

import { Channel, Device } from "./device";
import { Session, Status } from "./session";

export type ManagedEvent = { type: "connected" } | { type: "disconnected" };

interface EventSub {
  push(event: ManagedEvent): void;
  close(): void;
}

export class ManagedDevice {
  private dev: Device | null;
  private pinger: ReturnType<typeof setInterval>;
  private channel: Channel | null;
  private session: Session | null;
  private password: string | null = null;
  private _locked = false;
  private queue = new Queue();
  private subs: EventSub[] = [];
  private stopped = false;

  constructor(device: Device) {
    // set device
    this.dev = device;

    // start pinger
    this.pinger = setInterval(() => {
      void this.queue
        .run(async () => {
          if (!this.session) {
            return;
          }
          try {
            await this.session.ping(5000);
          } catch (e) {
          try {
            await this.session.end(0);
          } catch (e) {
            // ignore
          }
          this.session = null;
          }
        })
        .catch(() => {
          // ignore after stop/deactivate races
        });
    }, 5000);
  }

  device(): Device | null {
    return this.dev;
  }

  events(): AsyncIterable<ManagedEvent> {
    const buffer: ManagedEvent[] = [];
    let resolve: ((value: IteratorResult<ManagedEvent>) => void) | null = null;
    let done = this.stopped;

    const sub: EventSub = {
      push: (event: ManagedEvent) => {
        if (done) return;
        if (resolve) {
          const r = resolve;
          resolve = null;
          r({ value: event, done: false });
        } else if (buffer.length < 4) {
          buffer.push(event);
        }
      },
      close: () => {
        done = true;
        if (resolve) {
          const r = resolve;
          resolve = null;
          r({ value: undefined as unknown as ManagedEvent, done: true });
        }
      },
    };

    if (!done) {
      this.subs.push(sub);
    }

    const subs = this.subs;

    return {
      [Symbol.asyncIterator]() {
        return {
          next(): Promise<IteratorResult<ManagedEvent>> {
            if (done && buffer.length === 0) {
              return Promise.resolve({
                value: undefined as unknown as ManagedEvent,
                done: true,
              });
            }
            if (buffer.length > 0) {
              return Promise.resolve({ value: buffer.shift()!, done: false });
            }
            return new Promise((r) => {
              resolve = r;
            });
          },
          return(): Promise<IteratorResult<ManagedEvent>> {
            const idx = subs.indexOf(sub);
            if (idx >= 0) subs.splice(idx, 1);
            done = true;
            return Promise.resolve({
              value: undefined as unknown as ManagedEvent,
              done: true,
            });
          },
        };
      },
    };
  }

  async activate() {
    // check state
    if (this.active()) {
      return;
    }
    if (this.stopped) {
      throw new Error("managed device stopped");
    }

    // open channel
    const ch = await this.dev.open();
    this.channel = ch;

    // read lock status
    try {
      this.session = await this.newSession();
    } catch (e) {
      await ch.close();
      this.channel = null;
      throw e;
    }

    // emit connected
    this.emit({ type: "connected" });

    // watch for transport loss
    ch.done.then(() => {
      if (this.channel !== ch) return;
      void this.handleDisconnect();
    });
  }

  active(): boolean {
    return this.channel != null;
  }

  hasSession(): boolean {
    return this.session != null;
  }

  locked(): boolean {
    return this._locked;
  }

  async unlock(password: string): Promise<boolean> {
    // check state
    if (!this.active()) {
      throw new Error("device not active");
    }

    // unlock
    let unlocked!: boolean;
    await this.useSession(async (session) => {
      unlocked = await session.unlock(password, 1000);
    });

    // store password and update locked state if unlocked
    if (unlocked) {
      this.password = password;
      this._locked = false;
    }

    return unlocked;
  }

  async newSession(timeout: number = 5000): Promise<Session> {
    // check state
    if (this.stopped) {
      throw new Error("managed device stopped");
    }
    if (!this.active()) {
      throw new Error("device not active");
    }

    // open new session
    const session = await Session.open(this.channel, timeout);

    // get session status
    let status = await session.status(1000);

    // update locked state
    this._locked = (status & Status.locked) !== 0;

    // try to unlock if password is available and locked
    if (this.password && this._locked) {
      if (await session.unlock(this.password, 1000)) {
        this._locked = false;
      }
    }

    return session;
  }

  async useSession(fn: (session: Session) => Promise<void>) {
    await this.queue.run(async () => {
      // check state
      if (this.stopped) {
        throw new Error("managed device stopped");
      }
      if (!this.active()) {
        throw new Error("device not active");
      }

      // open session if absent
      if (!this.session) {
        this.session = await this.newSession();
      }

      // yield session
      try {
        await fn(this.session);
      } catch (e) {
        // close session
        try {
          await this.session.end(0);
        } catch (e) {
          // ignore
        }
        this.session = null;

        // rethrow
        throw e;
      }
    });
  }

  async deactivate() {
    // check state
    if (!this.active()) {
      return;
    }

    // capture state
    let channel = this.channel;
    let session = this.session;

    // clear state
    this.channel = null;
    this.session = null;

    // end session
    if (session) {
      try {
        await session.end(1000);
      } catch (e) {
        // ignore
      }
    }

    // close channel
    await channel.close();
  }

  async stop() {
    // deactivate
    await this.deactivate();

    // stop pinger
    clearInterval(this.pinger);

    // close all subscriber streams
    for (const sub of this.subs) {
      sub.close();
    }
    this.subs = [];
    this.stopped = true;

    // clear device
    this.dev = null;
    this.password = null;
  }

  private emit(event: ManagedEvent) {
    for (const sub of this.subs) {
      sub.push(event);
    }
  }

  private async handleDisconnect() {
    const channel = this.channel;
    if (!channel) {
      return;
    }

    this.session = null;
    this.channel = null;

    try {
      await channel.close();
    } catch {
      // ignore
    }

    this.emit({ type: "disconnected" });
  }
}
