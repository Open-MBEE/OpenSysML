// The per-platform packages: built from release binaries only against their
// published digests, and named exactly what this package's optionalDependencies ask for.

import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { createHash } from "node:crypto";
import { mkdirSync, mkdtempSync, readFileSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { test } from "node:test";
import { packageRoot } from "./support/service.js";

const SCRIPT = join(packageRoot, "scripts", "build-platform-packages.mjs");
const ASSETS = [
  "sysml-grpc-linux-amd64",
  "sysml-grpc-linux-arm64",
  "sysml-grpc-darwin-amd64",
  "sysml-grpc-darwin-arm64",
  "sysml-grpc-windows-amd64.exe",
];

test("every platform package is built, described and digest-checked", () => {
  const binaries = fakeRelease();
  const out = mkdtempSync(join(tmpdir(), "packages-"));
  const result = run(binaries, out);
  assert.equal(result.status, 0, result.stderr);

  const manifest = JSON.parse(readFileSync(join(out, "packages.json"), "utf8")) as {
    packages: { name: string; directory: string; digest: string }[];
  };
  assert.equal(manifest.packages.length, ASSETS.length);

  // The optional dependencies also carry the sigstore packages a signed manifest is
  // verified with, so only the platform packages are compared.
  const optional = Object.keys(
    (JSON.parse(readFileSync(join(packageRoot, "package.json"), "utf8")) as {
      optionalDependencies: Record<string, string>;
    }).optionalDependencies,
  ).filter((name) => name.startsWith("@opensysml/sysml-grpc-"));
  assert.deepEqual(
    manifest.packages.map((entry) => entry.name).sort(),
    optional.sort(),
  );

  for (const entry of manifest.packages) {
    const meta = JSON.parse(readFileSync(join(entry.directory, "package.json"), "utf8")) as {
      name: string;
      os: string[];
      cpu: string[];
      files: string[];
    };
    // npm selects the package by os/cpu, so those must be single and exact.
    assert.equal(meta.name, entry.name);
    assert.equal(meta.os.length, 1);
    assert.equal(meta.cpu.length, 1);
    assert.ok(entry.name.endsWith(`${meta.os[0]}-${meta.cpu[0]}`));
    assert.deepEqual(meta.files, ["bin", "README.md"]);
    assert.match(readFileSync(join(entry.directory, "README.md"), "utf8"), /SHA-256/);
  }
});

test("a binary whose digest does not match its sidecar is refused", () => {
  const binaries = fakeRelease();
  writeFileSync(join(binaries, `${ASSETS[0]}.sha256`), `${"0".repeat(64)}  ${ASSETS[0]}\n`);
  const result = run(binaries, mkdtempSync(join(tmpdir(), "packages-")));
  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /hashes to/);
});

test("a binary with no published digest is refused rather than trusted", () => {
  const binaries = fakeRelease({ sidecars: false });
  const result = run(binaries, mkdtempSync(join(tmpdir(), "packages-")));
  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /sha256 is missing/);
});

function run(binaries: string, out: string): { status: number | null; stderr: string } {
  const result = spawnSync(
    process.execPath,
    [SCRIPT, "--binaries", binaries, "--out", out, "--version", "0.0.0-test"],
    { encoding: "utf8" },
  );
  return { status: result.status, stderr: result.stderr };
}

/** A directory shaped like a release: one file per asset, with its .sha256 sidecar. */
function fakeRelease(options: { sidecars?: boolean } = {}): string {
  const dir = mkdtempSync(join(tmpdir(), "release-"));
  mkdirSync(dir, { recursive: true });
  for (const asset of ASSETS) {
    const bytes = Buffer.from(`#!/bin/sh\necho ${asset}\n`);
    writeFileSync(join(dir, asset), bytes);
    if (options.sidecars !== false) {
      const digest = createHash("sha256").update(bytes).digest("hex");
      writeFileSync(join(dir, `${asset}.sha256`), `${digest}  ${asset}\n`);
    }
  }
  return dir;
}
