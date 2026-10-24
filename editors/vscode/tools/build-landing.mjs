import { build } from "esbuild";
import { copyFile, mkdir } from "node:fs/promises";
import { fileURLToPath } from "node:url";

// The diagram bundles the browser client from its source in client/node, so the
// page needs no published package; the worker entry runs the module off the UI thread.
const clientBrowser = fileURLToPath(new URL("../../../client/node/src/browser/", import.meta.url));
const alias = { "@openmbee/opensysml/browser": `${clientBrowser}index.ts` };

const dev = process.argv.includes("--dev");
const outDir = "../../docs/assets/landing";

await mkdir(outDir, { recursive: true });
await copyFile("node_modules/libavoid-js/dist/libavoid.wasm", `${outDir}/libavoid.wasm`);
await copyFile("node_modules/libavoid-js/LICENSE", `${outDir}/libavoid-js.LICENSE.txt`);
const shared = {
  bundle: true,
  platform: "browser",
  target: "es2020",
  sourcemap: dev,
  minify: !dev,
  logLevel: "info",
  alias,
};
await build({
  ...shared,
  entryPoints: ["src/landing/main.ts"],
  outfile: `${outDir}/diagram.js`,
  format: "iife",
  // The client falls back to import.meta.url only where there is no document; the page has one.
  logOverride: { "empty-import-meta": "silent" },
});
await build({
  ...shared,
  entryPoints: [`${clientBrowser}wasm-worker.ts`],
  outfile: `${outDir}/wasm-worker.js`,
  format: "esm",
});
