// ParseSources: several documents as one model, against the real service.

import assert from "node:assert/strict";
import { mkdtempSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { before, test } from "node:test";
import { ParseError, SourceDocument, SymbolNotFoundError, connect } from "../src/node/index.js";
import { useServiceBinary } from "./support/service.js";

before(() => {
  useServiceBinary();
});

const LIBRARY = `package Lib {
    part def Engine {
        attribute power : ScalarValues::Real = 120.0;
    }
}
`;

const TOP = `package Top {
    private import Lib::*;
    part def Car {
        part engine : Engine;
    }
    part car : Car;
}
`;

const BROKEN = `package Broken {
    private import Lib::*;
    part def Boat {
        part motor : Missing;
    }
}
`;

test("several documents parse as one model resolving imports across them", async () => {
  await using connection = await connect();
  const model = await connection.parseSources([
    ["lib", LIBRARY],
    ["top", TOP],
  ]);
  assert.equal(model.documents.length, 2);
  assert.equal(model.roots.length, 2);
  const car = await model.symbol("car");
  assert.equal(car.name, "car");
});

test("a file path is a document among the inline ones", async () => {
  const dir = mkdtempSync(join(tmpdir(), "opensysml-src-"));
  const lib = join(dir, "lib.sysml");
  writeFileSync(lib, LIBRARY);
  await using connection = await connect();
  const model = await connection.parseSources([lib, ["top", TOP]]);
  assert.equal(model.documents.length, 2);
  await model.symbol("car");
});

test("SourceDocument objects parse beside the tuple form", async () => {
  await using connection = await connect();
  const model = await connection.parseSources([
    SourceDocument.inline("lib", LIBRARY),
    SourceDocument.inline("top", TOP),
  ]);
  await model.symbol("car");
});

test("a broken document reports diagnostics naming it", async () => {
  await using connection = await connect();
  const model = await connection.parseSources([
    ["lib", LIBRARY],
    ["broken", BROKEN],
  ]);
  assert.ok(model.diagnostics.length > 0);
  assert.ok(model.diagnostics.every((diagnostic) => diagnostic.file === "broken"));
});

test("strict turns a parse failure into a ParseError", async () => {
  await using connection = await connect();
  await assert.rejects(
    () => connection.parseSources([["broken", BROKEN]], { strict: true }),
    ParseError,
  );
});

test("a document whose name another takes is refused before it is sent", async () => {
  await using connection = await connect();
  await assert.rejects(
    () => connection.parseSources([["same", LIBRARY], ["same", TOP]]),
    RangeError,
  );
});

test("an unknown symbol is still a SymbolNotFoundError", async () => {
  await using connection = await connect();
  const model = await connection.parseSources([["top", TOP]]);
  await assert.rejects(() => model.symbol("nonexistent"), SymbolNotFoundError);
});
