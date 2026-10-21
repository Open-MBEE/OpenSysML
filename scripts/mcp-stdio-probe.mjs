#!/usr/bin/env node
// A minimal MCP client over stdio: starts the command given, performs the
// `initialize` handshake, asks for `tools/list`, and exits 0 only when the
// server answered both with tools to show. Everything else, a line on stdout
// that is not JSON-RPC included, fails loudly. Run:
//   node scripts/mcp-stdio-probe.mjs -- <command> [args...]
import { spawn } from "node:child_process";
import { createInterface } from "node:readline";

const separator = process.argv.indexOf("--");
const command = separator < 0 ? [] : process.argv.slice(separator + 1);
if (command.length === 0) {
  process.stderr.write("mcp-stdio-probe: usage: node scripts/mcp-stdio-probe.mjs -- <command> [args...]\n");
  process.exit(2);
}

const timeout = Number(process.env.MCP_PROBE_TIMEOUT_MS ?? "60000");
const child = spawn(command[0], command.slice(1), { stdio: ["pipe", "pipe", "inherit"] });
const pending = new Map();
let nextId = 1;

function request(method, params) {
  const id = nextId++;
  child.stdin.write(`${JSON.stringify({ jsonrpc: "2.0", id, method, params })}\n`);
  return new Promise((resolve, reject) => {
    pending.set(id, { resolve, reject, method });
  });
}

function notify(method, params) {
  child.stdin.write(`${JSON.stringify({ jsonrpc: "2.0", method, params })}\n`);
}

const lines = createInterface({ input: child.stdout });
lines.on("line", (line) => {
  let message;
  try {
    message = JSON.parse(line);
  } catch {
    fail(`stdout carried a line that is not JSON-RPC: ${JSON.stringify(line.slice(0, 200))}`);
    return;
  }
  if (message.jsonrpc !== "2.0") {
    fail(`stdout carried a message without jsonrpc "2.0": ${line.slice(0, 200)}`);
    return;
  }
  const waiting = message.id === undefined ? undefined : pending.get(message.id);
  if (waiting === undefined) {
    return; // a notification or a request from the server; not what is probed
  }
  pending.delete(message.id);
  if (message.error !== undefined) {
    waiting.reject(new Error(`${waiting.method} failed: ${JSON.stringify(message.error)}`));
  } else {
    waiting.resolve(message.result);
  }
});

let verdict = 1;
function fail(reason) {
  process.stderr.write(`mcp-stdio-probe: ${reason}\n`);
  verdict = 1;
  child.kill("SIGTERM");
}

const timer = setTimeout(() => fail(`no complete answer within ${timeout} ms`), timeout);

child.on("exit", (code, signal) => {
  clearTimeout(timer);
  if (verdict === 0 && code !== 0) {
    const ending = signal ?? `status ${code}`;
    process.stderr.write(`mcp-stdio-probe: the server ended with ${ending} after a clean probe\n`);
    verdict = 1;
  }
  process.exit(verdict);
});
child.on("error", (error) => fail(`could not start ${command[0]}: ${error.message}`));

try {
  const init = await request("initialize", {
    protocolVersion: "2025-06-18",
    capabilities: {},
    clientInfo: { name: "mcp-stdio-probe", version: "0" },
  });
  if (!init || typeof init.protocolVersion !== "string" || !init.serverInfo) {
    throw new Error(`initialize answered without protocolVersion and serverInfo: ${JSON.stringify(init)}`);
  }
  notify("notifications/initialized", {});
  const listed = await request("tools/list", {});
  if (!Array.isArray(listed?.tools) || listed.tools.length === 0) {
    throw new Error(`tools/list answered without tools: ${JSON.stringify(listed)}`);
  }
  process.stdout.write(
    `${init.serverInfo.name} ${init.serverInfo.version ?? ""} (protocol ${init.protocolVersion}): ` +
      `${listed.tools.length} tools, first ${listed.tools[0].name}\n`,
  );
  verdict = 0;
  child.stdin.end(); // the server ends when its client goes away
} catch (error) {
  fail(error.message);
}
