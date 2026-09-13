// Runs the extension's unit tests: every src/**/*.test.ts is bundled with esbuild,
// the way the extension itself is built, and handed to node's own test runner.
import { spawnSync } from "node:child_process";
import { mkdtemp, readdir, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join, relative, resolve } from "node:path";
import { build } from "esbuild";

const sourceDir = resolve(new URL("../src", import.meta.url).pathname);
const tests = (await readdir(sourceDir, { recursive: true }))
  .filter((name) => name.endsWith(".test.ts"))
  .map((name) => join(sourceDir, name));
if (tests.length === 0) {
  console.error("no *.test.ts under src/");
  process.exit(1);
}

const outdir = await mkdtemp(join(tmpdir(), "opensysml-vscode-test-"));
try {
  await build({
    entryPoints: tests,
    outdir,
    outbase: sourceDir,
    bundle: true,
    format: "esm",
    platform: "node",
    target: "node18",
    external: ["node:*"],
    sourcemap: "inline",
    logLevel: "warning",
  });

  const compiled = tests.map((path) => join(outdir, relative(sourceDir, path).replace(/\.ts$/, ".js")));
  const result = spawnSync(process.execPath, ["--test", "--enable-source-maps", ...compiled], { stdio: "inherit" });
  process.exitCode = result.status ?? 1;
} finally {
  await rm(outdir, { recursive: true, force: true });
}
