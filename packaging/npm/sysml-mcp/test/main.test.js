import assert from "node:assert/strict";
import { EventEmitter } from "node:events";
import { readFileSync } from "node:fs";
import { constants } from "node:os";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { test } from "node:test";
import { FORWARDED_SIGNALS, main, ownVersion, run, signalStatus } from "../lib/main.js";

const here = dirname(fileURLToPath(import.meta.url));
const node = process.execPath;

// A stand-in for process: it has a platform, an architecture and signal
// listeners, and `signal` delivers a signal to them as the OS would.
class FakeHost extends EventEmitter {
  constructor(platform = process.platform, arch = process.arch) {
    super();
    this.platform = platform;
    this.arch = arch;
  }
  signal(name) {
    this.emit(name);
  }
}

class Sink {
  text = "";
  write(chunk) {
    this.text += chunk;
  }
}

test("ownVersion is the version package.json declares", () => {
  const manifest = JSON.parse(readFileSync(join(here, "..", "package.json"), "utf8"));
  assert.equal(ownVersion(), manifest.version);
});

test("the child's exit code is the status", async () => {
  const stderr = new Sink();
  assert.equal(await run(node, ["-e", "process.exit(0)"], { stderr, host: new FakeHost() }), 0);
  assert.equal(await run(node, ["-e", "process.exit(7)"], { stderr, host: new FakeHost() }), 7);
  assert.equal(stderr.text, "");
});

test("the caller's arguments reach the child unchanged", async () => {
  const stderr = new Sink();
  const status = await run(
    node,
    ["-e", "process.exit(process.argv.slice(1).join(' ') === '-root . -v' ? 0 : 1)", "--", "-root", ".", "-v"],
    { stderr, host: new FakeHost() },
  );
  assert.equal(status, 0);
});

test("a binary that cannot start is reported on stderr with status 1", async () => {
  const stderr = new Sink();
  const missing = join(here, "no-such-binary");
  assert.equal(await run(missing, [], { stderr, host: new FakeHost() }), 1);
  assert.match(stderr.text, /@openmbee\/sysml-mcp: .*no-such-binary cannot be started \(ENOENT\)/);
});

test("signalStatus is 128 plus the signal's number", () => {
  assert.equal(signalStatus("SIGTERM"), 128 + constants.signals.SIGTERM);
  assert.equal(signalStatus("SIGINT"), 128 + constants.signals.SIGINT);
  assert.equal(signalStatus("SIGNOTREAL"), 1);
});

for (const signal of FORWARDED_SIGNALS) {
  test(`${signal} received by the shim ends the child and the status says so`, { skip: process.platform === "win32" }, async () => {
    const host = new FakeHost();
    const stderr = new Sink();
    // The child idles until a signal ends it; the shim's listener is in place
    // once spawn returned, which is before the first macrotask.
    const pending = run(node, ["-e", "setInterval(() => {}, 1000)"], { stderr, host });
    await new Promise((resolve) => setTimeout(resolve, 200));
    assert.equal(host.listenerCount(signal), 1);
    host.signal(signal);
    assert.equal(await pending, 128 + constants.signals[signal]);
    assert.equal(host.listenerCount(signal), 0, "the listener is removed once the child is gone");
  });
}

test("a child that handles the forwarded signal decides its own status", { skip: process.platform === "win32" }, async () => {
  const host = new FakeHost();
  const stderr = new Sink();
  const script = "process.on('SIGTERM', () => process.exit(3)); setInterval(() => {}, 1000)";
  const pending = run(node, ["-e", script], { stderr, host });
  await new Promise((resolve) => setTimeout(resolve, 500));
  host.signal("SIGTERM");
  assert.equal(await pending, 3);
});

test("main without a platform package for this machine says which one, on stderr, with status 1", async () => {
  const stderr = new Sink();
  const host = new FakeHost("sunos", "x64");
  assert.equal(await main([], { stderr, host }), 1);
  assert.match(stderr.text, /no release of sysml-mcp is built for sunos-x64/);
});

test("main runs the given binary with the arguments", async () => {
  const stderr = new Sink();
  assert.equal(await main(["-e", "process.exit(5)"], { stderr, host: new FakeHost(), binary: node }), 5);
  assert.equal(stderr.text, "");
});
