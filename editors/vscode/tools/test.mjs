// Runs the extension's unit tests: every *.test.ts under src/ and tools/ is bundled
// with esbuild, the way the extension itself is built, and handed to node's own test runner.
// The bundles land under out/ so a test's jsdom resolves from node_modules.
// With --coverage the runner goes through c8, which maps the bundles back to
// src/ through their inline source maps and writes coverage/lcov.info.
import { spawnSync } from "node:child_process";
import { mkdir, mkdtemp, readdir, rm } from "node:fs/promises";
import { createRequire } from "node:module";
import { join, relative } from "node:path";
import { fileURLToPath } from "node:url";
import { build } from "esbuild";

const extensionDir = fileURLToPath(new URL("..", import.meta.url));
const dirs = ["src", "tools"];
const listings = await Promise.all(dirs.map((dir) => readdir(join(extensionDir, dir), { recursive: true })));
const tests = dirs.flatMap((dir, i) => listings[i].filter((name) => name.endsWith(".test.ts")).map((name) => join(extensionDir, dir, name)));
if (tests.length === 0) {
  console.error("no *.test.ts under src/ or tools/");
  process.exit(1);
}

const outRoot = fileURLToPath(new URL("../out", import.meta.url));
await mkdir(outRoot, { recursive: true });
const outdir = await mkdtemp(join(outRoot, "test-"));
try {
  await build({
    entryPoints: tests,
    outdir,
    outbase: extensionDir,
    bundle: true,
    format: "esm",
    platform: "node",
    target: "node18",
    external: ["node:*", "jsdom"],
    alias: {
      "@openmbee/opensysml/browser": fileURLToPath(new URL("../../../client/node/src/browser/index.ts", import.meta.url)),
    },
    sourcemap: "inline",
    logLevel: "warning",
  });

  const compiled = tests.map((path) => join(outdir, relative(extensionDir, path).replace(/\.ts$/, ".js")));
  const args = ["--test", "--enable-source-maps", ...compiled];
  if (process.argv.includes("--coverage")) {
    // The bundles are *.test.js, which c8 excludes by default; excluding after the
    // remap drops the tests themselves and keeps the src/ files they bundle.
    const c8 = createRequire(import.meta.url).resolve("c8/bin/c8.js");
    args.unshift(c8, "--reporter=text", "--reporter=lcovonly", "--exclude-after-remap", process.execPath);
  }
  const result = spawnSync(process.execPath, args, { stdio: "inherit" });
  process.exitCode = result.status ?? 1;
} finally {
  await rm(outdir, { recursive: true, force: true });
}
