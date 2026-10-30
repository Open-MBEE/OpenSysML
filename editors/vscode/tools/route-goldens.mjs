// Bundles tools/route-goldens.ts the way the tests are bundled and runs it:
//   node tools/route-goldens.mjs [output-dir]   (a directory inside the repository checkout)
import { spawnSync } from "node:child_process";
import { mkdir } from "node:fs/promises";
import { fileURLToPath } from "node:url";

import { build } from "esbuild";

const entry = fileURLToPath(new URL("./route-goldens.ts", import.meta.url));
const outfile = fileURLToPath(new URL("../out/route-goldens.mjs", import.meta.url));
await mkdir(new URL("../out/", import.meta.url), { recursive: true });
await build({
  entryPoints: [entry],
  outfile,
  bundle: true,
  format: "esm",
  platform: "node",
  target: "node18",
  external: ["node:*"],
  logLevel: "warning",
});
const run = spawnSync(process.execPath, [outfile, ...process.argv.slice(2)], { stdio: "inherit" });
process.exit(run.status ?? 1);
