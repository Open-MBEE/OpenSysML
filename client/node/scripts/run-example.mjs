// Runs a compiled example, or every one of them in order.
//
// Usage: node scripts/run-example.mjs <name>|--all
// The examples run against $OPENSYSML_BINARY when it names one, and a service
// built from this checkout otherwise, as the service-backed tests do.

import { execFileSync, spawnSync } from "node:child_process";
import { existsSync, mkdirSync, readdirSync, renameSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const packageRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const repoRoot = resolve(packageRoot, "../..");
const built = join(packageRoot, "build", "examples");

const provided = process.env.OPENSYSML_BINARY;
if (provided === undefined || !existsSync(provided)) {
  const dir = join(packageRoot, "build", "service");
  const path = join(dir, process.platform === "win32" ? "sysml-grpc.exe" : "sysml-grpc");
  if (!existsSync(path)) {
    mkdirSync(dir, { recursive: true });
    const staged = `${path}.${process.pid}`;
    execFileSync("go", ["build", "-o", staged, "./cmd/sysml-grpc"], { cwd: repoRoot, stdio: "inherit" });
    renameSync(staged, path);
  }
  process.env.OPENSYSML_BINARY = path;
}

const names = readdirSync(built)
  .filter((entry) => /^\d\d-.*\.js$/.test(entry))
  .sort()
  .map((entry) => entry.replace(/\.js$/, ""));

const [wanted] = process.argv.slice(2);
if (wanted === undefined) {
  console.error(`usage: node scripts/run-example.mjs <name>|--all\nexamples: ${names.join(" ")}`);
  process.exit(2);
}

const selected = wanted === "--all" ? names : names.filter((name) => name.startsWith(wanted.replace(/\.ts$/, "")));
if (selected.length === 0) {
  console.error(`no example matches ${JSON.stringify(wanted)}; there are: ${names.join(" ")}`);
  process.exit(2);
}

for (const name of selected) {
  console.log(`\n=== ${name} ===`);
  const run = spawnSync(process.execPath, [join(built, `${name}.js`)], { stdio: "inherit" });
  if (run.status !== 0) {
    console.error(`${name} failed with status ${run.status ?? run.signal}`);
    process.exit(run.status ?? 1);
  }
}
