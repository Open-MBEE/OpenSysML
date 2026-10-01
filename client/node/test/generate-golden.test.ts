// Golden test: the module opensysml-generate renders for the packaged vehicle
// model, against the real service. Regenerate with UPDATE_GOLDEN=1.

import assert from "node:assert/strict";
import { existsSync, readFileSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { before, test } from "node:test";
import { connect } from "../src/node/index.js";
import { generateSource, modelStamp } from "../src/node/generate.js";
import { packageRoot, repoRoot, useServiceBinary } from "./support/service.js";

const FIXTURE = join(repoRoot, "internal/frontend/repl/testdata/vehicle_package.sysml");
const GOLDEN = join(packageRoot, "test", "golden", "vehicle_types.ts");

before(() => {
  useServiceBinary();
});

test("the generated module for the vehicle model matches the golden", async () => {
  const sourceText = readFileSync(FIXTURE, "utf8");
  await using connection = await connect();
  const model = await connection.load(FIXTURE);
  const source = await generateSource(model, sourceText);

  if (process.env["UPDATE_GOLDEN"] === "1" || !existsSync(GOLDEN)) {
    writeFileSync(GOLDEN, source);
  }
  assert.equal(source, readFileSync(GOLDEN, "utf8"));

  // The stamp records this exact source, so the golden tells what it came from.
  assert.ok(source.includes(`SYSML_MODEL_HASH = "sha256:${modelStamp(sourceText)}"`));
});
