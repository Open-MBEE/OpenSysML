import { build } from "esbuild";
import { copyFile, mkdir } from "node:fs/promises";

const dev = process.argv.includes("--dev");
const outDir = "../../docs/assets/landing";

await mkdir(outDir, { recursive: true });
await copyFile("node_modules/libavoid-js/dist/libavoid.wasm", `${outDir}/libavoid.wasm`);
await copyFile("node_modules/libavoid-js/LICENSE", `${outDir}/libavoid-js.LICENSE.txt`);
await build({
  entryPoints: ["src/landing/main.ts"],
  bundle: true,
  outfile: `${outDir}/diagram.js`,
  format: "iife",
  platform: "browser",
  target: "es2020",
  sourcemap: dev,
  minify: !dev,
  logLevel: "info",
});
