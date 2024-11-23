import { Queue } from "async-await-queue";

import { Channel, Device } from "./device";
import { Session, Status } from "./session";

export class ManagedDevice {
  private device: Device;
  private pinger: number;
  private channel: Channel | null;
  private session: Session | null;
  private password: string | null = null;
  private queue = new Queue();

  constructor(device: Device) {
    // set device
    this.device = device;

    // start pinger
    this.pinger = setInterval(async () => {
      if (this.active()) {
        await this.useSession(async (session) => {
          await session.ping(1000);
        });
      }
    }, 5000);
  }

  async activate() {
    // check state
    if (this.active()) {
      return;
    }

    // open channel
    this.channel = await this.device.open();
  }

  active(): Boolean {
    return this.channel != null;
  }

  async locked(): Promise<Boolean> {
    // check state
    if (!this.active()) {
      throw new Error("device not active");
    }

    // get status
    let status: Status;
    await this.useSession(async (session) => {
      status = await session.status(1000);
    });

    return (status & Status.locked) != 0;
  }

  async unlock(password: string): Promise<boolean> {
    // check state
    if (!this.active()) {
      throw new Error("device not active");
    }

    // unlock
    let unlocked: boolean;
    await this.useSession(async (session) => {
      unlocked = await session.unlock(password, 1000);
    });

    // store password if unlocked
    if (unlocked) {
      this.password = password;
    }

    return unlocked;
  }

  async newSession(): Promise<Session> {
    // check state
    if (!this.active()) {
      throw new Error("device not active");
    }

    // open new session
    const session = await Session.open(this.channel);

    // get session status
    let status = await session.status(1000);

    // try to unlock if password is available and locked
    if (this.password && status & Status.locked) {
      await session.unlock(this.password, 1000);
    }

    return session;
  }

  async useSession(fn: (session: Session) => Promise<void>) {
    await this.queue.run(async () => {
      // check state
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
        this.session.end(1000).then();
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
    try {
      await session.end(1000);
    } catch (e) {
      // ignore
    }

    // close channel
    channel.close();
  }

  async stop() {
    this.device = null;
    clearInterval(this.pinger);
    await this.deactivate();
  }
}
