class Queue<T> {
  private items: T[] = [];
  private head = 0;

  get length(): number {
    return this.items.length - this.head;
  }

  push(item: T): void {
    this.items.push(item);
  }

  shift(): T {
    const item = this.items[this.head++];
    if (this.head * 2 >= this.items.length) {
      this.items = this.items.slice(this.head);
      this.head = 0;
    }
    return item;
  }
}

class Chan<T> {
  private buffer = new Queue<T>();
  private senders = new Queue<{ value: T; resume: () => void }>();
  private receivers = new Queue<(value: T | undefined) => void>();
  private closed = false;

  constructor(private readonly capacity = 0) {}

  send(value: T): Promise<void> | void {
    if (this.receivers.length > 0) {
      this.receivers.shift()(value);
      return;
    }
    if (this.buffer.length < this.capacity) {
      this.buffer.push(value);
      return;
    }
    return new Promise((resume) => this.senders.push({ value, resume }));
  }

  recv(): Promise<T | undefined> | T | undefined {
    if (this.buffer.length > 0) {
      const value = this.buffer.shift();
      if (this.senders.length > 0) {
        const sender = this.senders.shift();
        this.buffer.push(sender.value);
        sender.resume();
      }
      return value;
    }
    if (this.senders.length > 0) {
      const sender = this.senders.shift();
      sender.resume();
      return sender.value;
    }
    if (this.closed) return undefined;
    return new Promise((resolve) => this.receivers.push(resolve));
  }

  close(): void {
    this.closed = true;
    while (this.receivers.length > 0) this.receivers.shift()(undefined);
  }
}

async function echo(input: Chan<number>, output: Chan<number>): Promise<void> {
  for (;;) {
    const v = await input.recv();
    if (v === undefined) break;
    await output.send(v + 1);
  }
  output.close();
}

async function produce(jobs: Chan<number>, n: number): Promise<void> {
  for (let i = 0; i < n; i++) {
    await jobs.send(i);
  }
  jobs.close();
}

async function work(jobs: Chan<number>, results: Chan<number>): Promise<void> {
  let sum = 0;
  for (;;) {
    const j = await jobs.recv();
    if (j === undefined) break;
    sum += j * 2;
  }
  await results.send(sum);
}

const ping = new Chan<number>();
const pong = new Chan<number>();
echo(ping, pong);
let x = 0;
for (let i = 0; i < 500000; i++) {
  await ping.send(x);
  x = (await pong.recv()) ?? 0;
}
ping.close();
console.log(x);

const workers = 4;
const jobs = new Chan<number>(128);
const results = new Chan<number>();
produce(jobs, 2000000);
for (let w = 0; w < workers; w++) {
  work(jobs, results);
}
let total = 0;
for (let w = 0; w < workers; w++) {
  total += (await results.recv()) ?? 0;
}
console.log(total);
