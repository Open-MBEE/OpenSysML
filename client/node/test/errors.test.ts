// What a failing call raises: the status a service error carries, the name a
// lookup could not find, and the options refused before anything is opened.

import assert from "node:assert/strict";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { before, test } from "node:test";
import {
  ModelFileNotFoundError,
  ModelNotFoundError,
  OpenSysMLError,
  ServiceError,
  SymbolNotFoundError,
  connect,
  fromRpcError,
  statusName,
} from "../src/node/index.js";
import { Code, ConnectError } from "@connectrpc/connect";
import { SAMPLE, useServiceBinary } from "./support/service.js";

before(() => {
  useServiceBinary();
});

test("a model the service no longer holds is a ModelNotFoundError with its status", async () => {
  await using connection = await connect();
  const missing = connection.model("0".repeat(64));
  const error = await missing.eval("2 + 2").then(
    () => undefined,
    (reason: unknown) => reason,
  );
  assert.ok(error instanceof ModelNotFoundError);
  assert.equal(error.code, "NOT_FOUND");
  assert.match(error.message, /model not found/);
  assert.ok(error.cause instanceof ConnectError);
});

test("a source file the service cannot read is a ModelFileNotFoundError", async () => {
  await using connection = await connect();
  await assert.rejects(
    () => connection.load(join(tmpdir(), "opensysml-no-such-model.sysml")),
    ModelFileNotFoundError,
  );
});

test("a lookup that fails names what it looked for and what the model has instead", async () => {
  await using connection = await connect();
  await using model = await connection.loads(SAMPLE);
  const error = await model.symbol("Wheeel").then(
    () => undefined,
    (reason: unknown) => reason,
  );
  assert.ok(error instanceof SymbolNotFoundError);
  assert.equal(error.symbolName, "Wheeel");
  assert.equal(error.suggestions[0], "Sample::Wheel");
  assert.match(error.message, /did you mean Sample::Wheel/);
});

test("a qualified name the model has not got reports that name, not the service's text", async () => {
  await using connection = await connect();
  await using model = await connection.loads(SAMPLE);
  const error = await model.symbol("Sample::Nope").then(
    () => undefined,
    (reason: unknown) => reason,
  );
  assert.ok(error instanceof SymbolNotFoundError);
  assert.equal(error.symbolName, "Sample::Nope");
  assert.deepEqual(error.suggestions, []);
});

test("options that are no options are refused before a service is started", async () => {
  await assert.rejects(() => connect({ encoding: "yaml" as "json" }), OpenSysMLError);
  await assert.rejects(() => connect({ timeoutMs: 0 }), OpenSysMLError);
  await assert.rejects(() => connect({ timeoutMs: -1 }), OpenSysMLError);
  await assert.rejects(() => connect({ timeoutMs: Number.NaN }), OpenSysMLError);
});

test("a handshake nothing answers names the service and the status it failed with", async () => {
  const error = await connect({ address: "127.0.0.1:1", timeoutMs: 2000 }).then(
    () => undefined,
    (reason: unknown) => reason,
  );
  assert.ok(error instanceof ServiceError);
  assert.equal(error.code, "UNAVAILABLE");
  assert.match(error.message, /127\.0\.0\.1:1.* did not answer/);
});

test("an RPC failure becomes the error its status names, and this client's errors pass through", () => {
  assert.equal(statusName(Code.Unavailable), "UNAVAILABLE");
  const unavailable = fromRpcError(new ConnectError("nothing there", Code.Unavailable));
  assert.ok(unavailable instanceof ServiceError);
  assert.equal(unavailable.code, "UNAVAILABLE");
  assert.match(unavailable.message, /service unavailable: nothing there/);

  const file = fromRpcError(new ConnectError("file not found: /nope", Code.NotFound));
  assert.ok(file instanceof ModelFileNotFoundError);

  const symbol = fromRpcError(new ConnectError("symbol not found: Demo::Nope", Code.NotFound));
  assert.ok(symbol instanceof SymbolNotFoundError);
  assert.equal(symbol.symbolName, "Demo::Nope");

  const own = new SymbolNotFoundError("Wheel");
  assert.equal(fromRpcError(own), own);
});
