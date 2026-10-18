import assert from "node:assert/strict";
import { test } from "node:test";

import { connectWasmHost, type WasmHost } from "../src/core/wasm.js";

/** A traced run as the combined module answers it: the service's ExecuteStateResponse. */
const traced = {
  statesVisited: ["begin", "opensysml"],
  trace: [
    { kind: "entry", time: { realValue: 0 }, machine: "Demo::Journey", state: "begin", text: "enter begin" },
    {
      kind: "choice",
      time: { realValue: 0 },
      machine: "Demo::Journey",
      alternatives: ["1->opensysml", "2->flexo"],
      taken: "1->opensysml",
      text: "chose 1->opensysml",
    },
    { kind: "transition", time: { realValue: 1 }, machine: "Demo::Journey", from: "begin", to: "opensysml", text: "begin -> opensysml" },
    { kind: "accept", time: { realValue: 1 }, machine: "Demo::Journey", event: "Commit", text: "accept Commit" },
  ],
  traceDropped: 0,
};

/** A module answering the handshake and one ExecuteState, recording what it was asked. */
function tracingHost(capabilities: string[], calls: Array<{ method: string; params: unknown }>): WasmHost {
  return {
    version: "trace-test",
    call(method: string, params: string) {
      calls.push({ method, params: JSON.parse(params) });
      const result = method === "GetServerInfo" ? { version: "trace-test", capabilities } : traced;
      return Promise.resolve(JSON.stringify({ jsonrpc: "2.0", id: null, result }));
    },
    close: () => Promise.resolve(),
  };
}

test("a traced executeState reads a module's DocumentEvent records through the typed connection", async () => {
  const calls: Array<{ method: string; params: unknown }> = [];
  const connection = await connectWasmHost(tracingHost(["state_trace", "schedule"], calls), {});
  const run = await connection.executeState("h1", "Demo::Journey", {
    events: ["Commit"],
    schedule: "seed:7",
    trace: true,
  });
  assert.deepEqual(calls.map(({ method }) => method), ["GetServerInfo", "ExecuteState"]);
  assert.deepEqual(calls[1].params, {
    modelHash: "h1",
    stateMachineSymbolId: "Demo::Journey",
    events: ["Commit"],
    schedule: "seed:7",
    trace: true,
  });
  assert.deepEqual(run.statesVisited, ["begin", "opensysml"]);
  assert.equal(run.traceDropped, 0);
  assert.deepEqual(
    run.trace.map((event) => [event.kind, event.state, event.fromState, event.toState, event.event, event.taken]),
    [
      ["entry", "begin", "", "", "", ""],
      ["choice", "", "", "", "", "1->opensysml"],
      ["transition", "", "begin", "opensysml", "", ""],
      ["accept", "", "", "", "Commit", ""],
    ],
  );
  assert.deepEqual(run.trace[1].alternatives, ["1->opensysml", "2->flexo"]);
  assert.equal(run.trace[2].time, 1);
  assert.equal(run.trace[3].text, "accept Commit");
  await connection.close();
});

test("a module without state_trace refuses a traced executeState before calling it", async () => {
  const calls: Array<{ method: string; params: unknown }> = [];
  const connection = await connectWasmHost(tracingHost(["file"], calls), {});
  await assert.rejects(connection.executeState("h1", "Demo::Journey", { events: ["Commit"], trace: true }), /state_trace/);
  assert.deepEqual(calls.map(({ method }) => method), ["GetServerInfo"]);
  await connection.close();
});
