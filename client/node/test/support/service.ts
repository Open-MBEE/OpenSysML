// Builds the service the integration tests run against, once per test process.

import { execFileSync } from "node:child_process";
import { existsSync, mkdirSync, renameSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { BINARY_ENV } from "../../src/node/index.js";

/** The package root, wherever the compiled test happens to live. */
export const packageRoot = resolve(dirname(fileURLToPath(import.meta.url)), "../../..");
export const repoRoot = resolve(packageRoot, "../..");

/** A binary the environment provides, read once so a test may change the variable. */
const provided = process.env[BINARY_ENV];

/**
 * The service to test against: one the environment provides, or a build of this
 * checkout made once.
 */
export function serviceBinary(): string {
  if (provided !== undefined && existsSync(provided)) {
    return provided;
  }
  const dir = join(packageRoot, "build", "service");
  const path = join(dir, process.platform === "win32" ? "sysml-grpc.exe" : "sysml-grpc");
  if (!existsSync(path)) {
    mkdirSync(dir, { recursive: true });
    // Build aside and move into place: test files run in parallel processes, and
    // one spawning the binary another is still writing fails with ETXTBSY.
    const staged = `${path}.${String(process.pid)}`;
    execFileSync("go", ["build", "-o", staged, "./cmd/sysml-grpc"], { cwd: repoRoot, stdio: "inherit" });
    renameSync(staged, path);
  }
  return path;
}

/** Points the client at the built service, as an installed platform package would. */
export function useServiceBinary(): void {
  process.env[BINARY_ENV] = serviceBinary();
}

/** A tiny model every test can parse. */
export const SAMPLE = `package Sample {
  part def Wheel {
    attribute radius : ScalarValues::Real = 0.3;
  }
  part def Car {
    part wheels : Wheel[4];
    attribute mass : ScalarValues::Real = 1500.0;
  }
}
`;
