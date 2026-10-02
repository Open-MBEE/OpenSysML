// Migrating a SysML v1 model: the verb, its report, the refusals either way,
// and writing the result out with its image files.

import assert from "node:assert/strict";
import {
  copyFileSync,
  existsSync,
  mkdirSync,
  mkdtempSync,
  readFileSync,
  renameSync,
  symlinkSync,
} from "node:fs";
import { tmpdir } from "node:os";
import { join, relative } from "node:path";
import { before, test } from "node:test";
import {
  InvalidRequestError,
  MIGRATED_NOT_CONVERTED,
  Migration,
  MigrationReport,
  connect,
  save,
} from "../src/node/index.js";
import { SAMPLE, repoRoot, useServiceBinary } from "./support/service.js";

const VEHICLE_XMI = join(repoRoot, "conformance", "fixtures", "vehicle.xmi");

before(() => {
  useServiceBinary();
});

test("a v1 model migrates from its path, with the summary and the counts", async () => {
  await using connection = await connect();
  const migration = await connection.migrate("sysml", { path: VEHICLE_XMI });
  assert.equal(migration.fromFormat, "xmi");
  assert.equal(migration.toFormat, "sysml");
  assert.match(migration.content, /part def Vehicle/);
  assert.match(migration.content, /attribute mass/);
  assert.equal(migration.experimental, true);
  assert.match(
    migration.experimentalNotice,
    /SysML v1 migration is experimental/,
  );
  assert.match(
    migration.report.summary,
    /^migrated 93 element\(s\): 77 mapped, 13 approximated, 3 unmapped/,
  );
  assert.equal(migration.report.mapped, 77);
  assert.equal(migration.report.approximated, 13);
  assert.equal(migration.report.unmapped, 3);
  assert.equal(migration.report.skipped, 2);
  assert.notEqual(migration.report.source, "");
  assert.equal(
    migration.report.entries.length,
    0,
    "the full ledger comes back only when asked",
  );
  assert.equal(migration.report.text, "");
  assert.equal(String(migration.report), migration.report.summary);
  assert.equal(migration.results, "");
  assert.equal(migration.files.size, 0);
  assert.equal(String(migration), migration.content);
});

test("inline bytes migrate under the form fromFormat names, answered canonically", async () => {
  await using connection = await connect();
  const migration = await connection.migrate(
    "sysml",
    { content: new Uint8Array(readFileSync(VEHICLE_XMI)) },
    { fromFormat: "uml" },
  );
  assert.equal(migration.fromFormat, "xmi");
  assert.match(migration.content, /part def Vehicle/);
});

test("the full report and the results index come back when asked for", async () => {
  await using connection = await connect();
  const migration = await connection.migrate(
    "ttl",
    { path: VEHICLE_XMI },
    { report: true, results: true },
  );
  assert.equal(migration.toFormat, "ttl");
  assert.equal(migration.report.entries.length, 95);
  assert.equal(migration.report.byVerdict("unmapped").length, 3);
  assert.equal(migration.report.byVerdict("skipped").length, 2);
  const entry = migration.report.entries[0];
  assert.notEqual(entry.id, "");
  assert.notEqual(entry.kind, "");
  assert.ok(
    ["mapped", "approximated", "unmapped", "skipped"].includes(entry.verdict),
  );
  assert.match(migration.report.text, /^# SysML v1 to v2 migration report/);
  assert.equal(String(migration.report), migration.report.text);
  assert.notEqual(migration.results, "");
});

test("convert refuses a v1 model by its extension and by its format, pointing at migrate", async () => {
  await using connection = await connect();
  await assert.rejects(
    () => connection.convert("sysml", { path: VEHICLE_XMI }),
    (error: unknown) => {
      assert.ok(error instanceof InvalidRequestError);
      assert.equal(error.code, "INVALID_ARGUMENT");
      assert.equal(
        error.message,
        `${VEHICLE_XMI} ${MIGRATED_NOT_CONVERTED}; call migrate() with the same source`,
      );
      return true;
    },
  );
  for (const fromFormat of ["xmi", "uml", "mdzip", "XMI", " mdzip "]) {
    await assert.rejects(
      () => connection.convert("sysml", { content: "<xmi/>" }, { fromFormat }),
      (error: unknown) => {
        assert.ok(error instanceof InvalidRequestError);
        assert.match(
          error.message,
          /^the source is a SysML v1 model, which is migrated, not converted/,
        );
        return true;
      },
    );
  }
});

test("a v1 form is read as the service spells it, in any case and padding", async () => {
  await using connection = await connect();
  const migration = await connection.migrate(
    "sysml",
    { path: VEHICLE_XMI },
    { fromFormat: " XMI " },
  );
  assert.equal(migration.fromFormat, "xmi");
  assert.match(migration.content, /part def Vehicle/);
});

test("migrate refuses a v2 source, pointing at convert", async () => {
  await using connection = await connect();
  await assert.rejects(
    () =>
      connection.migrate(
        "sysml",
        { content: Buffer.from(SAMPLE) },
        { fromFormat: "sysml" },
      ),
    (error: unknown) => {
      assert.ok(error instanceof InvalidRequestError);
      assert.equal(error.code, "INVALID_ARGUMENT");
      assert.match(
        error.message,
        /is sysml input, which is converted, not migrated/,
      );
      assert.match(error.message, /call convert\(\) with the same source/);
      return true;
    },
  );
});

test("the service refuses inline content that names no form, and a v1 target", async () => {
  await using connection = await connect();
  const content = new Uint8Array(readFileSync(VEHICLE_XMI));
  await assert.rejects(
    () => connection.migrate("sysml", { content }),
    (error: unknown) => {
      assert.ok(error instanceof InvalidRequestError);
      assert.match(error.message, /from_format/);
      return true;
    },
  );
  await assert.rejects(
    () => connection.migrate("xmi", { path: VEHICLE_XMI }),
    (error: unknown) => {
      assert.ok(error instanceof InvalidRequestError);
      assert.match(error.message, /xmi/);
      return true;
    },
  );
});

test("exactly one source and at most one layout are given", async () => {
  await using connection = await connect();
  await assert.rejects(
    () => connection.migrate("sysml", {} as never),
    RangeError,
  );
  await assert.rejects(
    () =>
      connection.migrate("sysml", {
        path: VEHICLE_XMI,
        content: new Uint8Array(),
      }),
    RangeError,
  );
  await assert.rejects(
    () =>
      connection.migrate(
        "sysml",
        { path: VEHICLE_XMI },
        { layoutPath: "layout.xml", layoutContent: "<mtip/>" },
      ),
    RangeError,
  );
});

test("the migration warns that it is experimental", async () => {
  const warnings: string[] = [];
  const listener = (warning: Error): void => {
    if (warning.name === "ExperimentalFeatureWarning") {
      warnings.push(warning.message);
    }
  };
  process.on("warning", listener);
  try {
    await using connection = await connect();
    await connection.migrate("sysml", { path: VEHICLE_XMI });
    await new Promise((resolve) => setImmediate(resolve));
  } finally {
    process.off("warning", listener);
  }
  assert.equal(warnings.length, 1);
  assert.match(warnings[0] ?? "", /SysML v1 migration is experimental/);
});

test("save writes a migration and its image files beside it", async () => {
  const dir = mkdtempSync(join(tmpdir(), "opensysml-migrate-"));
  await using connection = await connect();
  const migrated = await connection.migrate("sysml", { path: VEHICLE_XMI });
  const path = join(dir, "Vehicle.sysml");
  assert.equal(await save(migrated, path), migrated);
  assert.equal(readFileSync(path, "utf8"), migrated.content);

  const png = new Uint8Array([0x89, 0x50, 0x4e, 0x47]);
  const withImages = new Migration({
    content: migrated.content,
    fromFormat: migrated.fromFormat,
    toFormat: migrated.toFormat,
    report: new MigrationReport({
      source: migrated.report.source,
      exporter: migrated.report.exporter,
      summary: migrated.report.summary,
      mapped: migrated.report.mapped,
      approximated: migrated.report.approximated,
      unmapped: migrated.report.unmapped,
      skipped: migrated.report.skipped,
      entries: [],
      text: "",
    }),
    results: "",
    files: new Map([["Vehicle_images/overview.png", png]]),
    sourcePath: migrated.sourcePath,
    experimentalNotice: migrated.experimentalNotice,
  });
  const nested = join(dir, "out", "Vehicle.sysml");
  await import("node:fs/promises").then((fs) => fs.mkdir(join(dir, "out")));
  await save(withImages, nested);
  assert.equal(readFileSync(nested, "utf8"), migrated.content);
  const image = join(dir, "out", "Vehicle_images", "overview.png");
  assert.ok(existsSync(image));
  assert.deepEqual(new Uint8Array(readFileSync(image)), png);
});

test("save refuses to replace the v1 model with its migration", async () => {
  const dir = mkdtempSync(join(tmpdir(), "opensysml-migrate-"));
  const source = join(dir, "Vehicle.xmi");
  copyFileSync(VEHICLE_XMI, source);
  await using connection = await connect();
  const migration = await connection.migrate("sysml", { path: source });
  assert.equal(migration.sourcePath, source);
  await assert.rejects(
    () => save(migration, source),
    (error: unknown) => {
      assert.ok(error instanceof RangeError);
      assert.equal(
        error.message,
        `${source} names the model being migrated; the v1 model would be replaced by its migration`,
      );
      return true;
    },
  );
  assert.deepEqual(readFileSync(source), readFileSync(VEHICLE_XMI));
  const inline = await connection.migrate(
    "sysml",
    { content: new Uint8Array(readFileSync(VEHICLE_XMI)) },
    { fromFormat: "xmi" },
  );
  assert.equal(inline.sourcePath, "");
  await save(inline, source);
  assert.equal(readFileSync(source, "utf8"), inline.content);

  const moved = join(dir, "Moved.xmi");
  copyFileSync(VEHICLE_XMI, moved);
  const ofMoved = await connection.migrate("sysml", { path: moved });
  renameSync(moved, moved + ".bak");
  await save(ofMoved, moved);
  assert.equal(
    readFileSync(moved, "utf8"),
    ofMoved.content,
    "a vacated source path protects nothing",
  );
  assert.deepEqual(readFileSync(moved + ".bak"), readFileSync(VEHICLE_XMI));
});

test("save refuses an image that would escape the directory or replace the model, writing nothing", async () => {
  const dir = mkdtempSync(join(tmpdir(), "opensysml-migrate-"));
  await using connection = await connect();
  const migrated = await connection.migrate("sysml", { path: VEHICLE_XMI });
  const path = join(dir, "Vehicle.sysml");
  const withFiles = (files: [string, Uint8Array][]): Migration =>
    new Migration({
      content: migrated.content,
      fromFormat: migrated.fromFormat,
      toFormat: migrated.toFormat,
      report: migrated.report,
      results: "",
      files: new Map(files),
      sourcePath: migrated.sourcePath,
      experimentalNotice: migrated.experimentalNotice,
    });
  const png = new Uint8Array([0x89, 0x50, 0x4e, 0x47]);
  for (const escaping of [
    "../escaped.png",
    "images/../../escaped.png",
    "/tmp/escaped.png",
    "images//escaped.png",
  ]) {
    await assert.rejects(
      () => save(withFiles([[escaping, png]]), path),
      (error: unknown) => {
        assert.ok(error instanceof RangeError);
        assert.equal(
          error.message,
          `the migration's image ${escaping} would land outside ${dir}`,
        );
        return true;
      },
    );
  }
  await assert.rejects(
    () => save(withFiles([["Vehicle.sysml", png]]), path),
    (error: unknown) => {
      assert.ok(error instanceof RangeError);
      assert.equal(
        error.message,
        `the migration's image Vehicle.sysml would replace ${path}`,
      );
      return true;
    },
  );
  assert.ok(!existsSync(path), "nothing is written when an image is refused");
  assert.ok(!existsSync(join(dir, "escaped.png")));

  const outside = mkdtempSync(join(tmpdir(), "opensysml-outside-"));
  symlinkSync(outside, join(dir, "images"), "dir");
  mkdirSync(join(dir, "nested"));
  symlinkSync(outside, join(dir, "nested", "link"), "dir");
  for (const linked of [
    "images/escaped.png",
    "nested/link/deeper/escaped.png",
  ]) {
    await assert.rejects(
      () => save(withFiles([[linked, png]]), path),
      (error: unknown) => {
        assert.ok(error instanceof RangeError);
        assert.equal(
          error.message,
          `the migration's image ${linked} would land outside ${dir}`,
        );
        return true;
      },
    );
  }
  assert.ok(!existsSync(path));
  assert.ok(!existsSync(join(outside, "escaped.png")));
  assert.ok(!existsSync(join(outside, "deeper")));

  mkdirSync(join(dir, "dangling"));
  symlinkSync(
    join(outside, "created.png"),
    join(dir, "dangling", "file.png"),
    "file",
  );
  symlinkSync(join(outside, "missing"), join(dir, "dangling", "dir"), "dir");
  for (const dangling of ["dangling/file.png", "dangling/dir/escaped.png"]) {
    await assert.rejects(
      () => save(withFiles([[dangling, png]]), path),
      (error: unknown) => {
        assert.ok(error instanceof RangeError);
        assert.equal(
          error.message,
          `the migration's image ${dangling} would land outside ${dir}`,
        );
        return true;
      },
    );
  }
  assert.ok(!existsSync(join(outside, "created.png")));
  assert.ok(!existsSync(join(outside, "missing")));

  mkdirSync(join(dir, "linked"));
  symlinkSync(
    join(dir, "linked", "real.png"),
    join(dir, "linked", "alias.png"),
    "file",
  );
  await assert.rejects(
    () => save(withFiles([["linked/alias.png", png]]), path),
    (error: unknown) => {
      assert.ok(error instanceof RangeError);
      assert.equal(
        error.message,
        `the migration's image linked/alias.png would be written through a symbolic link at ${join(dir, "linked", "alias.png")}`,
      );
      return true;
    },
  );
  assert.ok(!existsSync(path));
  assert.ok(!existsSync(join(dir, "linked", "real.png")));
});

test("a relative source path is remembered absolute", async () => {
  const dir = mkdtempSync(join(tmpdir(), "opensysml-migrate-"));
  const source = join(dir, "Vehicle.xmi");
  copyFileSync(VEHICLE_XMI, source);
  await using connection = await connect();
  const migration = await connection.migrate("sysml", {
    path: relative(process.cwd(), source),
  });
  assert.equal(migration.sourcePath, source);
});

test("the source path is fixed before the working directory can move", async () => {
  const dir = mkdtempSync(join(tmpdir(), "opensysml-migrate-"));
  const source = join(dir, "Vehicle.xmi");
  copyFileSync(VEHICLE_XMI, source);
  const other = mkdtempSync(join(tmpdir(), "opensysml-migrate-"));
  copyFileSync(VEHICLE_XMI, join(other, "Vehicle.xmi"));
  const cwd = process.cwd();
  try {
    process.chdir(dir);
    await using connection = await connect();
    const pending = connection.migrate("sysml", { path: "Vehicle.xmi" });
    process.chdir(other);
    const migration = await pending;
    assert.equal(migration.sourcePath, source);
  } finally {
    process.chdir(cwd);
  }
});

test("save follows an image into directories as deep as the file system allows", async () => {
  const dir = mkdtempSync(join(tmpdir(), "opensysml-migrate-"));
  const path = join(dir, "Vehicle.sysml");
  await using connection = await connect();
  const migrated = await connection.migrate("sysml", { path: VEHICLE_XMI });
  const deep =
    Array.from({ length: 70 }, (_, index) => `d${index}`).join("/") +
    "/deep.png";
  const png = new Uint8Array([0x89, 0x50, 0x4e, 0x47]);
  const migration = new Migration({
    content: migrated.content,
    fromFormat: migrated.fromFormat,
    toFormat: migrated.toFormat,
    report: migrated.report,
    results: "",
    files: new Map([[deep, png]]),
    sourcePath: migrated.sourcePath,
    experimentalNotice: migrated.experimentalNotice,
  });
  await save(migration, path);
  assert.deepEqual(
    new Uint8Array(readFileSync(join(dir, ...deep.split("/")))),
    png,
  );
});
