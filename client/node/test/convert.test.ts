// Converting a model to notation, Turtle and API JSON, and writing it out.

import assert from "node:assert/strict";
import { existsSync, mkdtempSync, readFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { before, test } from "node:test";
import {
  type Conversion,
  InvalidRequestError,
  MissingCapabilityError,
  Model,
  connect,
  save,
} from "../src/node/index.js";
import { fakeConnection } from "./support/fake.js";
import { SAMPLE, useServiceBinary } from "./support/service.js";

before(() => {
  useServiceBinary();
});

test("API JSON names library references separately and opts into legacy stubs", async () => {
  await using connection = await connect();
  const model = await connection.loads("package P { private import SI::*; attribute time = 20 [s]; }");
  const plain = await model.convert("api-json");
  const second = plain.libraryReferences.find((ref) => ref.qualifiedName === "SI::second");
  assert.ok(second);
  assert.equal(second.id, "0c539f7c-20a1-520d-bae0-f3f855517aea");
  assert.equal(second.type, "AttributeUsage");
  const plainElements = JSON.parse(plain.content) as Record<string, unknown>[];
  assert.equal(plainElements.filter((element) => Boolean(element.isLibraryElement)).length, 0);
  const legacy = await model.convert("api-json", { includeLibraryStubs: true });
  const legacyElements = JSON.parse(legacy.content) as Record<string, unknown>[];
  const stub = legacyElements.find((element) => element["@id"] === second.id);
  assert.ok(stub);
  assert.equal(stub.qualifiedName, "SI::second");
  assert.equal(stub.isLibraryElement, true);
  assert.deepEqual(legacy.libraryReferences, plain.libraryReferences);
});

test("API JSON spells derived ids as uuids when asked, over the same elements", async () => {
  await using connection = await connect();
  const model = await connection.loads(SAMPLE);
  const qualified = await model.convert("api-json");
  const explicit = await model.convert("api-json", { idForm: "qualified" });
  const uuid = await model.convert("api-json", { idForm: "uuid" });
  assert.equal(explicit.content, qualified.content);
  const names = (conversion: Conversion) =>
    (JSON.parse(conversion.content) as Record<string, unknown>[]).map((element) => element.qualifiedName);
  assert.deepEqual(names(uuid), names(qualified));
  const ids = (JSON.parse(uuid.content) as Record<string, string>[]).map((element) => element["@id"]);
  const uuidForm = /^[0-9a-f]{8}-[0-9a-f]{4}-5[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;
  assert.ok(ids.length > 0 && ids.every((id) => uuidForm.test(id)));
  assert.notEqual(uuid.content, qualified.content);
  await assert.rejects(
    () => model.convert("api-json", { idForm: "camel" as "uuid" }),
    (error: unknown) => error instanceof InvalidRequestError && /idForm|id_form/.test((error as Error).message),
  );
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

test("save converts with the id form and library stubs convert takes", async () => {
  const dir = mkdtempSync(join(tmpdir(), "opensysml-save-"));
  await using connection = await connect();
  const model = await connection.loads("package P { private import SI::*; attribute time = 20 [s]; }");
  const uuidPath = join(dir, "uuid.json");
  const uuid = await save(model, uuidPath, { idForm: "uuid" });
  assert.equal(readFileSync(uuidPath, "utf8"), uuid.content);
  assert.equal(uuid.content, (await model.convert("api-json", { idForm: "uuid" })).content);
  assert.notEqual(uuid.content, (await model.convert("api-json")).content);
  const stubsPath = join(dir, "stubs.json");
  const stubs = await save(model, stubsPath, { includeLibraryStubs: true });
  assert.equal(readFileSync(stubsPath, "utf8"), stubs.content);
  assert.equal(stubs.content, (await model.convert("api-json", { includeLibraryStubs: true })).content);
  const elements = JSON.parse(stubs.content) as Record<string, unknown>[];
  assert.ok(elements.some((element) => element.isLibraryElement === true));
});

test("save refuses an id form for a notation target before writing", async () => {
  const dir = mkdtempSync(join(tmpdir(), "opensysml-save-"));
  await using connection = await connect();
  const model = await connection.loads(SAMPLE);
  const path = join(dir, "out.sysml");
  await assert.rejects(
    () => save(model, path, { idForm: "uuid" }),
    (error: unknown) => error instanceof InvalidRequestError && /id_form|idForm/.test((error as Error).message),
  );
  assert.equal(existsSync(path), false);
});

test("save gates library stubs on the service's capability, as convert does", async () => {
  const dir = mkdtempSync(join(tmpdir(), "opensysml-save-"));
  const path = join(dir, "out.json");
  const unable = await fakeConnection(["convert"]);
  await assert.rejects(
    () => save(Model.adopt(unable, "hash"), path, { includeLibraryStubs: true }),
    (error: unknown) =>
      error instanceof MissingCapabilityError && error.capability === "convert_library_stubs",
  );
  assert.equal(existsSync(path), false);
  await unable.close();

  let seen: { toFormat?: string; idForm?: string; includeLibraryStubs?: boolean } | undefined;
  const able = await fakeConnection(["convert", "convert_library_stubs"], (method, input) => {
    assert.equal(method, "Convert");
    seen = input as typeof seen;
    return {
      content: "[]",
      fromFormat: "sysml",
      toFormat: "api-json",
      diagnostics: [],
      error: "",
      experimental: false,
      experimentalNotice: "",
      libraryReferences: [],
    };
  });
  await save(Model.adopt(able, "hash"), path, { idForm: "uuid", includeLibraryStubs: true });
  assert.ok(seen !== undefined);
  assert.equal(seen.toFormat, "api-json");
  assert.equal(seen.idForm, "uuid");
  assert.equal(seen.includeLibraryStubs, true);
  assert.equal(readFileSync(path, "utf8"), "[]");
  await able.close();
});
