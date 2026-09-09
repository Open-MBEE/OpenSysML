// Runs every example against a real service, so the examples cannot rot.

import assert from "node:assert/strict";
import { execFile } from "node:child_process";
import { readdirSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { promisify } from "node:util";
import { before, test } from "node:test";
import { useServiceBinary } from "./support/service.js";

const run = promisify(execFile);
const examples = resolve(dirname(fileURLToPath(import.meta.url)), "../examples");

before(() => {
  useServiceBinary();
});

const names = readdirSync(examples)
  .filter((entry) => /^\d\d-.*\.js$/.test(entry))
  .sort();

test("the examples directory holds the examples the README names", () => {
  assert.ok(names.length >= 6, `found ${String(names.length)} examples in ${examples}`);
});

for (const name of names) {
  test(`example ${name} runs clean`, async () => {
    const { stdout } = await run(process.execPath, [join(examples, name)], {
      env: process.env,
      timeout: 120_000,
    });
    assert.match(stdout, /\S/);
  });
}

// `npm run example <name>` goes through this script, so it is run the same way.
const runExample = resolve(examples, "../../scripts/run-example.mjs");

test("run-example.mjs runs the example named and reports which", async () => {
  const name = names[0].replace(/\.js$/, "");
  const { stdout } = await run(process.execPath, [runExample, name], { env: process.env, timeout: 120_000 });
  assert.match(stdout, new RegExp(`=== ${name} ===`));
  assert.doesNotMatch(stdout, /=== \d\d-.* ===[\s\S]*=== \d\d-.* ===/, "more than one example ran");
});

test("run-example.mjs refuses a name matching no example and no name at all", async () => {
  await assert.rejects(run(process.execPath, [runExample, "99-nothing"], { env: process.env }), (error: unknown) => {
    const failure = error as { code?: number; stderr?: string };
    assert.equal(failure.code, 2);
    assert.match(failure.stderr ?? "", /no example matches "99-nothing"/);
    return true;
  });
  await assert.rejects(run(process.execPath, [runExample], { env: process.env }), (error: unknown) => {
    const failure = error as { code?: number; stderr?: string };
    assert.equal(failure.code, 2);
    assert.match(failure.stderr ?? "", /usage: node scripts\/run-example.mjs/);
    return true;
  });
});
