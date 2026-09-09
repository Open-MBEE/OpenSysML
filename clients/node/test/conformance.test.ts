// Runs the conformance suite through the runner in process, the way
// `npm run conformance` does, so the runner is tested against a real service.

import assert from "node:assert/strict";
import { mkdtempSync, readFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { after, before, test } from "node:test";
import { parseOptions, runSuite } from "../conformance/main.js";
import { MUTATIONS } from "../conformance/mutations.js";
import type { Report } from "../conformance/runner.js";
import { repoRoot, serviceBinary } from "./support/service.js";

const suite = join(repoRoot, "conformance");
let workDir: string;
let full: Report;

before(() => {
  workDir = mkdtempSync(join(tmpdir(), "conformance-test-"));
});

after(() => {
  rmSync(workDir, { recursive: true, force: true });
});

test("the suite passes over every protocol and writes the report", async () => {
  const lines: string[] = [];
  const reportPath = join(workDir, "report.json");
  const report = await runSuite({
    dir: suite,
    binary: serviceBinary(),
    repo: repoRoot,
    protocols: ["grpc", "connect", "connect-json"],
    report: reportPath,
    verbose: true,
    allowSkips: true,
    log: (line) => lines.push(line),
  });
  assert.equal(report.failed, 0, lines.join("\n"));
  assert.equal(report.errored, 0, lines.join("\n"));
  assert.ok(report.passed > 0, "no scenario passed");
  assert.equal(report.protocols.length, 3);
  assert.equal(report.total, report.passed + report.skipped);
  assert.ok(lines.some((line) => /\bpass\b/i.test(line)), "the log names no passing scenario");

  const written = JSON.parse(readFileSync(reportPath, "utf8")) as Report;
  assert.deepEqual(written, report);
  full = report;
});

test("a filter runs only the matching scenarios", async () => {
  const report = await runSuite({
    dir: suite,
    binary: serviceBinary(),
    repo: repoRoot,
    protocols: ["connect"],
    run: "^parse/",
    verbose: false,
    allowSkips: true,
    log: () => {},
  });
  assert.ok(report.total > 0, "the filter matched nothing");
  assert.ok(report.total < full.total, "the filter ran the whole suite");
  assert.equal(report.failed + report.errored, 0);
});

test("a skipped scenario fails the run unless skips are allowed", async () => {
  await assert.rejects(
    runSuite({
      dir: suite,
      binary: serviceBinary(),
      repo: repoRoot,
      protocols: ["connect"],
      verbose: false,
      allowSkips: false,
      log: () => {},
    }),
    /skipped/,
  );
});

test("an unknown protocol is refused", async () => {
  await assert.rejects(
    runSuite({
      dir: suite,
      binary: serviceBinary(),
      repo: repoRoot,
      protocols: ["carrier-pigeon"],
      verbose: false,
      allowSkips: true,
      log: () => {},
    }),
    /unknown protocol "carrier-pigeon"/,
  );
});

// A runner that passes against a deliberately broken client tests nothing, so
// each mutation must make the suite fail.
for (const mutation of Object.keys(MUTATIONS)) {
  test(`the runner catches the ${mutation} mutation`, async () => {
    await assert.rejects(
      runSuite({
        dir: suite,
        binary: serviceBinary(),
        repo: repoRoot,
        protocols: ["connect"],
        verbose: false,
        allowSkips: true,
        mutate: mutation,
        log: () => {},
      }),
      /scenarios failed/,
    );
  });
}

test("the command line mirrors cmd/conformance's flags", () => {
  const options = parseOptions(["--binary", "bin/sysml-grpc", "--protocols", " grpc, connect ", "--allow-skips", "-v", "--mutate", "shift-integer"]);
  assert.equal(options.binary, "bin/sysml-grpc");
  assert.deepEqual(options.protocols, ["grpc", "connect"]);
  assert.equal(options.allowSkips, true);
  assert.equal(options.verbose, true);
  assert.equal(options.mutate, "shift-integer");
  assert.equal(options.dir, "conformance");
  assert.equal(options.repo, ".");

  assert.deepEqual(parseOptions([]).protocols, ["grpc", "connect", "connect-json"]);
  assert.throws(() => parseOptions(["--protocols", " , "]), /at least one protocol/);
  assert.throws(() => parseOptions(["--protocols", "grpc,grpc"]), /more than once/);
  assert.throws(() => parseOptions(["--mutate", "nothing"]), /unknown --mutate "nothing"/);
});
