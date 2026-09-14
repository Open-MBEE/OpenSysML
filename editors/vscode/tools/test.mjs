// Runs the extension's unit tests: every src/**/*.test.ts is bundled with esbuild,
// the way the extension itself is built, and handed to node's own test runner.
// The bundles land under out/ so a test's jsdom resolves from node_modules.
import { spawnSync } from "node:child_process";
import { mkdir, mkdtemp, readdir, rm } from "node:fs/promises";
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

const outRoot = resolve(new URL("../out", import.meta.url).pathname);
await mkdir(outRoot, { recursive: true });
const outdir = await mkdtemp(join(outRoot, "test-"));
try {
  await build({
    entryPoints: tests,
    outdir,
    outbase: sourceDir,
    bundle: true,
    format: "esm",
    platform: "node",
    target: "node18",
    external: ["node:*", "jsdom"],
    sourcemap: "inline",
    logLevel: "warning",
  });

  const compiled = tests.map((path) => join(outdir, relative(sourceDir, path).replace(/\.ts$/, ".js")));
  const result = spawnSync(process.execPath, ["--test", "--enable-source-maps", ...compiled], { stdio: "inherit" });
  process.exitCode = result.status ?? 1;
} finally {
  await rm(outdir, { recursive: true, force: true });
}
