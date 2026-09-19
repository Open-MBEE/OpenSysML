// The private child: a sysml-grpc this process started and only it can reach.
// It reports its kernel-assigned port on stdout, and exits at end of file on the
// stdin pipe this process holds open — which SIGKILL closes and an exit hook does not.

import { spawn, type ChildProcessWithoutNullStreams } from "node:child_process";
import { ServiceStartError } from "../core/errors.js";
import { resolveBinary, type Binary } from "./binary.js";

const START_TIMEOUT_MS = 10_000;
const STOP_TIMEOUT_MS = 5_000;
const STDERR_LINES_KEPT = 20;

/** The one private child of this thread, and the connections holding it. */
let shared: PrivateService | undefined;
/** A start in flight, so concurrent connect() calls share one child. */
let starting: Promise<PrivateService> | undefined;

/** A sysml-grpc child of this process, shared by every connection that needs one. */
export class PrivateService {
  readonly address: string;
  readonly binary: Binary;
  /** Connections holding it; the last one released stops it. */
  refs = 0;

  private readonly child: ChildProcessWithoutNullStreams;
  private readonly stderr: string[];
  private ended: boolean;

  private constructor(init: {
    child: ChildProcessWithoutNullStreams;
    binary: Binary;
    address: string;
    stderr: string[];
  }) {
    this.child = init.child;
    this.binary = init.binary;
    this.address = init.address;
    this.stderr = init.stderr;
    this.ended = false;
    this.child.once("exit", () => {
      this.ended = true;
    });
  }

  /** The process id of the child, which the orphan-safety test reads. */
  get pid(): number | undefined {
    return this.child.pid;
  }

  /** Whether the child is still there to be used. */
  get alive(): boolean {
    return !this.ended && this.child.exitCode === null && this.child.signalCode === null;
  }

  /** The last lines the child wrote to stderr, for an error message. */
  get log(): readonly string[] {
    return this.stderr;
  }

  /** Drops one hold, stopping the child when it was the last. */
  async release(): Promise<void> {
    this.refs -= 1;
    if (this.refs > 0) {
      return;
    }
    if (shared === this) {
      shared = undefined;
    }
    await this.stop();
  }

  /**
   * Stops the child. Closing the pipe would end it on its own; it is signalled as
   * well so a connection closed in a long-lived process does not wait on the read.
   */
  async stop(): Promise<void> {
    if (!this.alive) {
      this.child.stdin.destroy();
      return;
    }
    // Waiting for the child is this program's work again, so its handle is
    // referenced for as long as the wait lasts.
    this.child.ref();
    const exited = new Promise<void>((resolve) => {
      this.child.once("exit", () => {
        resolve();
      });
    });
    this.child.stdin.end();
    this.child.kill("SIGTERM");
    const killed = await withTimeout(exited, STOP_TIMEOUT_MS);
    if (!killed) {
      this.child.kill("SIGKILL");
      await exited;
    }
    this.child.stdout.destroy();
    this.child.stderr.destroy();
    this.child.stdin.destroy();
  }

  /** Starts a child and waits for the address it bound. */
  static async start(): Promise<PrivateService> {
    const binary = await resolveBinary();
    const child = spawn(
      binary.path,
      ["-port", "0", "-health-port", "0", "-report-address", "-exit-with-parent"],
      {
        stdio: ["pipe", "pipe", "pipe"],
        // A new session (a new process group on Windows), so a Ctrl-C meant for
        // this process does not reach the child mid-call.
        detached: true,
        windowsHide: true,
      },
    );

    const stderr = tail(child, STDERR_LINES_KEPT);
    const address = firstLine(child);
    let reported: string;
    try {
      // The handles stay referenced until the address arrives: this read is the
      // program's work, and unreferencing it first would let Node exit mid-start.
      reported = await address;
    } catch (cause) {
      child.stdin.end();
      child.kill("SIGKILL");
      throw new ServiceStartError(startFailure(binary, stderr, cause), { cause });
    }
    // From here the child is not this program's work: a referenced handle would
    // stop a script that forgot close() from ever exiting.
    child.unref();
    unrefHandle(child.stdin);
    unrefHandle(child.stdout);
    unrefHandle(child.stderr);
    return new PrivateService({
      child,
      binary,
      address: reported,
      stderr,
    });
  }
}

/** Takes a hold on this thread's private child, starting one when there is none. */
export async function acquirePrivateService(): Promise<PrivateService> {
  for (;;) {
    const existing = shared;
    if (existing?.alive) {
      existing.refs += 1;
      return existing;
    }
    starting ??= PrivateService.start()
      .then((service) => {
        shared = service;
        return service;
      })
      .finally(() => {
        starting = undefined;
      });
    await starting;
  }
}

/** The private child this thread holds, for tests and diagnostics. */
export function currentPrivateService(): PrivateService | undefined {
  return shared;
}

function startFailure(binary: Binary, stderr: readonly string[], cause: unknown): string {
  const why = cause instanceof Error ? cause.message : String(cause);
  const log = stderr.length === 0 ? "" : `\n  it wrote: ${stderr.join("\n            ")}`;
  return `${binary.path} did not report an address it is serving: ${why}${log}`;
}

/** Resolves with the child's first stdout line, then drains the rest. */
function firstLine(child: ChildProcessWithoutNullStreams): Promise<string> {
  return new Promise<string>((resolve, reject) => {
    let buffered = "";
    let settled = false;
    const timer = setTimeout(() => {
      finish(() => {
        reject(new Error(`no address within ${START_TIMEOUT_MS} ms`));
      });
    }, START_TIMEOUT_MS);
    timer.unref();

    const finish = (settle: () => void): void => {
      if (settled) {
        return;
      }
      settled = true;
      clearTimeout(timer);
      child.stdout.off("data", onData);
      // The child must never block writing to a full pipe, so keep reading.
      child.stdout.resume();
      settle();
    };

    const onData = (chunk: Buffer | string): void => {
      buffered += chunk.toString();
      const newline = buffered.indexOf("\n");
      if (newline === -1) {
        return;
      }
      const line = buffered.slice(0, newline).trim();
      finish(() => {
        if (line === "") {
          reject(new Error("its first line of output was empty"));
        } else {
          resolve(line);
        }
      });
    };

    child.stdout.on("data", onData);
    child.once("error", (error) => {
      finish(() => {
        reject(error);
      });
    });
    child.once("exit", (code, signal) => {
      finish(() => {
        reject(new Error(`it exited (code ${code ?? "none"}, signal ${signal ?? "none"}) first`));
      });
    });
  });
}

/** Stops a stdio pipe from holding the event loop open; they are sockets, which unref. */
function unrefHandle(stream: unknown): void {
  const handle = stream as { unref?: () => void };
  handle.unref?.();
}

/** Keeps the last `lines` lines the child wrote to stderr. */
function tail(child: ChildProcessWithoutNullStreams, lines: number): string[] {
  const kept: string[] = [];
  let buffered = "";
  child.stderr.on("data", (chunk: Buffer | string) => {
    buffered += chunk.toString();
    const parts = buffered.split("\n");
    buffered = parts.pop() ?? "";
    for (const part of parts) {
      kept.push(part);
      if (kept.length > lines) {
        kept.shift();
      }
    }
  });
  return kept;
}

async function withTimeout(promise: Promise<void>, ms: number): Promise<boolean> {
  let timer: NodeJS.Timeout | undefined;
  const timeout = new Promise<false>((resolve) => {
    timer = setTimeout(() => {
      resolve(false);
    }, ms);
    timer.unref();
  });
  try {
    return await Promise.race([promise.then(() => true), timeout]);
  } finally {
    if (timer !== undefined) {
      clearTimeout(timer);
    }
  }
}
