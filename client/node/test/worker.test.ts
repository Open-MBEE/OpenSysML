// Worker threads: the shared child is per thread, since the module state that
// holds it is, and a worker's child is stopped when that worker closes it.

import assert from "node:assert/strict";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { after, before, test } from "node:test";
import { Worker } from "node:worker_threads";
import { connect, currentPrivateService } from "../src/node/index.js";
import { BINARY_ENV } from "../src/node/index.js";
import { useServiceBinary } from "./support/service.js";

const HELPER = join(dirname(fileURLToPath(import.meta.url)), "support", "worker-service.js");

before(() => {
  useServiceBinary();
});

after(() => {
  Reflect.deleteProperty(process.env, BINARY_ENV);
});

test("a worker thread starts its own child, not the main thread's", async () => {
  await using main = await connect();
  assert.ok(!main.isClosed);
  const ours = currentPrivateService()?.pid;
  assert.equal(typeof ours, "number");

  const theirs = await workerPid();
  assert.equal(typeof theirs, "number");
  assert.notEqual(theirs, ours);
  // The worker closed its connection, so its child is gone and ours is not.
  assert.ok(await gone(theirs as number));
  assert.ok(currentPrivateService()?.alive);
});

/** Runs the worker helper and resolves with the pid it reports. */
function workerPid(): Promise<unknown> {
  return new Promise<unknown>((resolve, reject) => {
    const worker = new Worker(HELPER);
    let reported: unknown;
    worker.on("message", (message: { pid?: unknown }) => {
      reported = message.pid;
    });
    worker.once("error", reject);
    worker.once("exit", () => {
      resolve(reported);
    });
  });
}

async function gone(pid: number): Promise<boolean> {
  const deadline = Date.now() + 5_000;
  for (;;) {
    try {
      process.kill(pid, 0);
    } catch {
      return true;
    }
    if (Date.now() > deadline) {
      return false;
    }
    await new Promise<void>((resolve) => setTimeout(resolve, 50));
  }
}
