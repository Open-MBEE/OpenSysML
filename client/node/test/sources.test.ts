// ParseSources: several documents as one model, against the real service.

import assert from "node:assert/strict";
import { mkdtempSync, readFileSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { before, test } from "node:test";
import {
  MissingCapabilityError,
  ParseError,
  SourceDocument,
  SymbolNotFoundError,
  connect,
  save,
} from "../src/node/index.js";
import { CAPABILITY_PARSE_SOURCES_AFFECTED } from "../src/core/capabilities.js";
import { ALL_CAPABILITIES, fakeConnection } from "./support/fake.js";
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

// An OpenSysML notation extension (`choice`), a warning by default and an error under strict conformance.
const EXTENDED = `package P {
  state def S {
    entry; then a;
    state a;
    state b;
    choice c;
    transition a_c first a then c;
    transition c_b first c then b;
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

function recordingParseSources(): {
  requests: Record<string, unknown>[];
  answer: (method: string, input: unknown) => unknown;
} {
  const requests: Record<string, unknown>[] = [];
  return {
    requests,
    answer: (method, input) => {
      if (method !== "ParseSources") {
        throw new Error(`the fake service answers nothing for ${method}`);
      }
      const request = input as { documents: { name?: string; filePath?: string }[]; baseModelHash: string };
      requests.push(request);
      return {
        modelHash: "fake",
        roots: request.documents.map((doc) => ({
          id: doc.name ?? doc.filePath ?? "",
          name: "",
          kind: "Namespace",
          metadata: {},
          childIds: [],
          attributes: [],
          specializations: [],
        })),
        diagnostics: [],
        error: "",
        affected: request.baseModelHash === "" ? [] : ["top"],
      };
    },
  };
}

test("baseModelHash reaches the wire and the affected documents come back", async () => {
  const { requests, answer } = recordingParseSources();
  await using connection = await fakeConnection([...ALL_CAPABILITIES, CAPABILITY_PARSE_SOURCES_AFFECTED], answer);
  const related = await connection.parseSources([["lib", LIBRARY], ["top", TOP]], { baseModelHash: "base-hash" });
  const fresh = await connection.parseSources([["lib", LIBRARY], ["top", TOP]]);
  assert.deepEqual(
    requests.map((request) => request.baseModelHash),
    ["base-hash", ""],
  );
  assert.deepEqual(related.affected, ["top"]);
  assert.deepEqual(fresh.affected, []);
});

test("baseModelHash is refused before the call when parse_sources_affected is absent", async () => {
  const { requests, answer } = recordingParseSources();
  await using connection = await fakeConnection(ALL_CAPABILITIES, answer);
  await assert.rejects(
    connection.parseSources([["lib", LIBRARY]], { baseModelHash: "base-hash" }),
    (error: unknown) =>
      error instanceof MissingCapabilityError && error.capability === CAPABILITY_PARSE_SOURCES_AFFECTED,
  );
  assert.equal(requests.length, 0, "the request was sent to a service that would ignore it");
});

test("a base model names the documents an edit reaches", async () => {
  // Names of this test's own: the service relates two models document by
  // document once it has seen their set more than once, and a model it first
  // answered fresh, under another test's names, cannot be a base.
  const documents = (top: string): [string, string][] => [
    ["affected-lib", LIBRARY],
    ["affected-top", top],
  ];
  await using connection = await connect();
  await connection.parseSources(documents(`${TOP}// first\n`));
  const base = await connection.parseSources(documents(TOP));
  assert.deepEqual(base.affected, []);
  const same = await connection.parseSources(documents(TOP), { baseModelHash: base.hash });
  assert.equal(same.hash, base.hash);
  assert.deepEqual(same.affected, []);
  const edited = await connection.parseSources(documents(TOP.replace("part car", "part sedan")), {
    baseModelHash: base.hash,
  });
  assert.notEqual(edited.hash, base.hash);
  assert.deepEqual(edited.affected, ["affected-top"]);
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
  assert.equal(model.documents.length, 2);
  assert.equal((await model.symbol("car")).name, "car");
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

test("strictConformance makes an OpenSysML-only notation the error the default parse reports as a warning", async () => {
  await using connection = await connect();
  const lenient = await connection.parseSources([["extended", EXTENDED]]);
  assert.equal(lenient.errors.length, 0);
  assert.ok(lenient.diagnostics.some((diagnostic) => diagnostic.severity === "warning"));
  const strict = await connection.parseSources([["extended", EXTENDED]], { strictConformance: true });
  assert.ok(strict.errors.length > 0);
  assert.throws(() => strict.raiseForErrors(), ParseError);
  const loaded = await connection.loads(EXTENDED, { strictConformance: true });
  assert.ok(loaded.errors.length > 0);
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

test("a file load names its path as the model's sourcePath", async () => {
  await using connection = await connect();
  const dir = mkdtempSync(join(tmpdir(), "opensysml-src-"));
  const path = join(dir, "top.sysml");
  writeFileSync(path, TOP);
  const model = await connection.load(path);
  assert.equal(model.sourcePath, path);
  assert.deepEqual(model.documents, [path]);
});

test("an inline load names no sourcePath and no documents", async () => {
  await using connection = await connect();
  const model = await connection.loads(TOP);
  assert.equal(model.sourcePath, undefined);
  assert.deepEqual(model.documents, []);
});

test("parseSources never names a sourcePath, even for one document", async () => {
  await using connection = await connect();
  const dir = mkdtempSync(join(tmpdir(), "opensysml-src-"));
  const lib = join(dir, "lib.sysml");
  writeFileSync(lib, LIBRARY);
  const oneFile = await connection.parseSources([lib]);
  assert.equal(oneFile.sourcePath, undefined);
  assert.deepEqual(oneFile.documents, [lib]);
  const oneInline = await connection.parseSources([SourceDocument.inline("top", TOP)]);
  assert.equal(oneInline.sourcePath, undefined);
  assert.deepEqual(oneInline.documents, ["top"]);
  const mixed = await connection.parseSources([lib, ["top", TOP]]);
  assert.equal(mixed.sourcePath, undefined);
  assert.equal(mixed.documents.length, 2);
});

test("save of a file load still writes the converted content", async () => {
  await using connection = await connect();
  const dir = mkdtempSync(join(tmpdir(), "opensysml-src-"));
  const path = join(dir, "top.sysml");
  const written = join(dir, "out.sysml");
  writeFileSync(path, TOP);
  const model = await connection.load(path);
  const conversion = await model.convert("sysml");
  await save(conversion, written);
  assert.equal(readFileSync(written, "utf8"), conversion.content);
});
