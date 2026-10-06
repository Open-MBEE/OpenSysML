// End to end: a module generated from a real parse is compiled by the repo's
// TypeScript and reads typed features of a real instantiate() result.

import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { after, before, test } from "node:test";
import { connect } from "../src/node/index.js";
import { generateSource } from "../src/node/generate.js";
import { packageRoot, useServiceBinary } from "./support/service.js";
import type { Instance } from "../src/core/model.js";
import type { RationalValue } from "../src/core/values.js";

const MODEL = `package Demo {
	part def Engine {
		attribute power = 300.0;
	}

	part def Vehicle {
		attribute mass = 1500.0;
		part engine : Engine;
	}

	part vehicle : Vehicle;
}
`;

interface GeneratedModule {
  Vehicle: new (instance: Instance, resolve?: (id: bigint) => Instance | undefined) => {
    mass: RationalValue;
    engine: { power: RationalValue };
  };
}

let workDir: string;

before(() => {
  useServiceBinary();
  workDir = mkdtempSync(join(packageRoot, "build", "typed-runtime-"));
});

after(() => {
  rmSync(workDir, { recursive: true, force: true });
});

test("a generated module compiles and reads an instantiated model", async () => {
  await using connection = await connect();
  const model = await connection.loads(MODEL);
  assert.ok(model.ok, model.errors.map((diagnostic) => diagnostic.message).join(", "));

  const source = await generateSource(model, MODEL);
  // Written inside the package so the generated @openmbee/opensysml import
  // resolves the way it will for a consumer of the published package.
  const tsPath = join(workDir, "demo_types.ts");
  writeFileSync(tsPath, source);
  execFileSync(
    "npx",
    [
      "tsc",
      tsPath,
      "--strict",
      "--module",
      "nodenext",
      "--moduleResolution",
      "nodenext",
      "--target",
      "es2022",
      "--outDir",
      workDir,
      "--rootDir",
      workDir,
      "--skipLibCheck",
    ],
    { cwd: packageRoot },
  );

  const generated = (await import(join(workDir, "demo_types.js"))) as GeneratedModule;

  const tree = await model.instantiate("vehicle");
  const vehicle = new generated.Vehicle(tree.root, (id) => tree.byId(id));
  assert.deepEqual(vehicle.mass, { numerator: 1500n, denominator: 1n });
  assert.deepEqual(vehicle.engine.power, { numerator: 300n, denominator: 1n });
});
