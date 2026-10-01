// Converting a model to notation, Turtle and API JSON, and writing it out.

import assert from "node:assert/strict";
import { mkdtempSync, readFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { before, test } from "node:test";
import { InvalidRequestError, connect, save } from "../src/node/index.js";
import { SAMPLE, useServiceBinary } from "./support/service.js";

before(() => {
  useServiceBinary();
});

test("a model writes itself out in notation", async () => {
  await using connection = await connect();
  const model = await connection.loads(SAMPLE);
  const conversion = await model.toSysml();
  assert.equal(conversion.toFormat, "sysml");
  assert.match(conversion.content, /part def Car/);
  assert.equal(conversion.experimental, false);
});

test("a path converts to notation without being loaded first", async () => {
  const dir = mkdtempSync(join(tmpdir(), "opensysml-cvt-"));
  const path = join(dir, "car.sysml");
  const { writeFileSync } = await import("node:fs");
  writeFileSync(path, SAMPLE);
  await using connection = await connect();
  const conversion = await connection.convert("sysml", { path });
  assert.match(conversion.content, /part def Wheel/);
});

test("inline content converts too", async () => {
  await using connection = await connect();
  const conversion = await connection.convert(
    "sysml",
    { content: SAMPLE },
    { fromFormat: "sysml" },
  );
  assert.match(conversion.content, /part def Car/);
});

test("a model converts by hash", async () => {
  await using connection = await connect();
  const model = await connection.loads(SAMPLE);
  const conversion = await connection.convert("sysml", { modelHash: model.hash });
  assert.match(conversion.content, /part def Car/);
});

test("exactly one source must be given", async () => {
  await using connection = await connect();
  await assert.rejects(() => connection.convert("sysml", {} as never), RangeError);
  await assert.rejects(
    () => connection.convert("sysml", { path: "a.sysml", modelHash: "h" }),
    RangeError,
  );
});

test("an unknown target format is an invalid request", async () => {
  await using connection = await connect();
  const model = await connection.loads(SAMPLE);
  await assert.rejects(() => model.convert("brainfuck"), InvalidRequestError);
});

test("save writes the format the path names", async () => {
  const dir = mkdtempSync(join(tmpdir(), "opensysml-save-"));
  await using connection = await connect();
  const model = await connection.loads(SAMPLE);
  const path = join(dir, "out.sysml");
  const conversion = await save(model, path);
  assert.equal(readFileSync(path, "utf8"), conversion.content);
  assert.match(conversion.content, /part def Car/);
});

test("save writes a conversion's content verbatim", async () => {
  const dir = mkdtempSync(join(tmpdir(), "opensysml-save-"));
  await using connection = await connect();
  const model = await connection.loads(SAMPLE);
  const conversion = await model.toSysml();
  const path = join(dir, "out.sysml");
  await save(conversion, path);
  assert.equal(readFileSync(path, "utf8"), conversion.content);
});
