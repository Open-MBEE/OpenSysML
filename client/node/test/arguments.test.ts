// Guards the public string entry points reach before any call is made.

import assert from "node:assert/strict";
import { before, test } from "node:test";
import { SourceDocument, connect } from "../src/node/index.js";
import { useServiceBinary } from "./support/service.js";

before(() => {
  useServiceBinary();
});

const MODEL = `package Demo { part def Wheel; }`;

test("a non-string argument is a TypeError naming its parameter", async () => {
  await using connection = await connect();
  const model = await connection.loads(MODEL);
  for (const [call, param, kind] of [
    [async () => connection.load(42 as unknown as string), "path", "number"],
    [async () => connection.loads(null as unknown as string), "source", "null"],
    [async () => model.eval(42 as unknown as string), "expression", "number"],
    [async () => model.symbol(42 as unknown as string), "name", "number"],
    [async () => model.symbolById(null as unknown as string), "id", "null"],
    [async () => model.find(false as unknown as string), "name", "boolean"],
    [async () => model.instantiate(7 as unknown as string), "name", "number"],
  ] as const) {
    await assert.rejects(call, (error: unknown) => {
      assert.ok(error instanceof TypeError, `${param}: ${String(error)}`);
      assert.match(
        (error).message,
        new RegExp(`^${param} must be a string, got ${kind}`),
      );
      return true;
    });
  }
  assert.throws(
    () => SourceDocument.inline("doc", 42 as unknown as string),
    (error: unknown) => {
      assert.ok(error instanceof TypeError);
      assert.match((error).message, /content must be a string, got number/);
      return true;
    },
  );
  await assert.rejects(
    async () => connection.parseSources([[42 as unknown as string, MODEL]]),
    TypeError,
  );
});

test("a lone UTF-16 surrogate in source text is refused before the call", async () => {
  await using connection = await connect();
  await assert.rejects(
    () => connection.loads("package P { part def A\uD800; }"),
    (error: unknown) => {
      assert.ok(error instanceof RangeError);
      assert.match((error).message, /lone UTF-16 surrogate/);
      return true;
    },
  );
  assert.throws(
    () => SourceDocument.inline("doc", "package P {\uDFFF }"),
    RangeError,
  );
  // A well-formed pair is fine.
  const model = await connection.loads("package P { part def \u{1F600}Odd; }");
  assert.ok(model.parsed);
});
