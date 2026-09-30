// The package manifest agrees with the constants the code derives names from:
// a rename in package.json without src/core/package.ts (or the reverse) fails here.

import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { test } from "node:test";
import { PACKAGE_NAME, PLATFORM_PACKAGE_PREFIX } from "../src/core/package.js";
import { packageRoot } from "./support/service.js";

const manifest = JSON.parse(
  readFileSync(join(packageRoot, "package.json"), "utf8"),
) as { name: string; version: string; optionalDependencies: Record<string, string> };

test("package.json names the package PACKAGE_NAME describes", () => {
  assert.equal(manifest.name, PACKAGE_NAME);
});

test("optionalDependencies name exactly the five platform packages at this version", () => {
  const expected = [
    "linux-x64",
    "linux-arm64",
    "darwin-x64",
    "darwin-arm64",
    "win32-x64",
  ].map((suffix) => `${PLATFORM_PACKAGE_PREFIX}${suffix}`);

  const platforms = Object.entries(manifest.optionalDependencies).filter(([name]) =>
    name.startsWith(PLATFORM_PACKAGE_PREFIX),
  );
  assert.deepEqual(
    platforms.map(([name]) => name).sort(),
    expected.sort(),
  );
  // npm installs the exact version the release publishes the platform packages at.
  for (const [, version] of platforms) {
    assert.equal(version, manifest.version);
  }
});
