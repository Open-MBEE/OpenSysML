// Orphan safety: the child is reaped when its parent dies, including a SIGKILL
// no exit hook could observe, and it does not hold a finished script open.

import assert from "node:assert/strict";
import { spawn, type ChildProcess } from "node:child_process";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { test } from "node:test";
import { serviceBinary } from "./support/service.js";

const HELPER = join(dirname(fileURLToPath(import.meta.url)), "support", "hold-service.js");

test("SIGKILLing the parent leaves no orphaned service", async (t) => {
  if (process.platform === "win32") {
    t.skip("Windows has no SIGKILL; the pipe guarantee is the same but untestable this way");
    return;
  }
  serviceBinary();
  const parent = spawn(process.execPath, [HELPER, "wait"], { stdio: ["ignore", "pipe", "inherit"] });
  const child = await reportedPid(parent);
  assert.ok(alive(child));

  // Not SIGTERM: SIGKILL runs no handler, so only the kernel closing the stdin
  // pipe the parent held can end the child.
  parent.kill("SIGKILL");
  await exit(parent);
  assert.ok(await gone(child), `sysml-grpc ${child} outlived the process that started it`);
});

test("a script that never closes its connection still exits, and its service goes with it", async () => {
  serviceBinary();
  const parent = spawn(process.execPath, [HELPER, "return"], {
    stdio: ["ignore", "pipe", "inherit"],
  });
  const child = await reportedPid(parent);
  // The child must not keep the event loop alive: this exits on its own.
  const code = await exit(parent);
  assert.equal(code, 0);
  assert.ok(await gone(child), `sysml-grpc ${child} outlived the script that started it`);
});

/** The pid the helper reports on its first line of output. */
async function reportedPid(parent: ChildProcess): Promise<number> {
  const line = await new Promise<string>((resolve, reject) => {
    let buffered = "";
    parent.stdout?.on("data", (chunk: Buffer) => {
      buffered += chunk.toString();
      const newline = buffered.indexOf("\n");
      if (newline !== -1) {
        resolve(buffered.slice(0, newline));
      }
    });
    parent.once("error", reject);
    parent.once("exit", (code, signal) => {
      reject(new Error(`the helper exited (code ${code ?? "none"}, signal ${signal ?? "none"})`));
    });
  });
  const reported: unknown = JSON.parse(line);
  assert.ok(typeof reported === "object" && reported !== null && "pid" in reported);
  const { pid } = reported as { pid: number };
  assert.equal(typeof pid, "number");
  return pid;
}

function alive(pid: number): boolean {
  try {
    process.kill(pid, 0);
    return true;
  } catch {
    return false;
  }
}

/** Whether `pid` is gone within a few seconds of its parent dying. */
async function gone(pid: number): Promise<boolean> {
  const deadline = Date.now() + 5_000;
  while (Date.now() < deadline) {
    if (!alive(pid)) {
      return true;
    }
    await sleep(50);
  }
  return false;
}

function exit(child: ChildProcess): Promise<number | null> {
  return new Promise<number | null>((resolve) => {
    child.once("exit", (code) => {
      resolve(code);
    });
  });
}

function sleep(ms: number): Promise<void> {
  return new Promise<void>((resolve) => {
    setTimeout(resolve, ms);
  });
}
