export class AsyncQueue<Type> {
  private queue: Type[] = [];
  private waiters: ((item: Type | null) => void)[] = [];

  push(item: Type) {
    // add to back
    this.queue.push(item);

    // process queue
    while (this.waiters.length > 0 && this.queue.length > 0) {
      const resolve = this.waiters.shift();
      resolve(this.queue.shift());
    }
  }

  pop(timeout: number): Promise<Type | null> {
    // check if there is an item in the queue
    if (this.queue.length > 0) {
      return Promise.resolve(this.queue.shift());
    }

    return new Promise((resolve: (value: Type | null) => void): void => {
      // add waiter
      this.waiters.push(resolve);

      // handle timeout
      if (timeout > 0) {
        setTimeout((): void => {
          if (this.waiters.includes(resolve)) {
            const index: number = this.waiters.indexOf(resolve);
            this.waiters.splice(index, 1);
            resolve(null);
          }
        }, timeout);
      }
    });
  }
}
