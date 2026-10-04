import { context } from "esbuild";
import { copyFile, mkdir } from "node:fs/promises";

const production = process.argv.includes("--production");
const watch = process.argv.includes("--watch");

// The webview fetches libavoid's WASM at runtime rather than bundling it.
await mkdir("dist", { recursive: true });
await copyFile("node_modules/libavoid-js/dist/libavoid.wasm", "dist/libavoid.wasm");

// Two bundles: the extension runs in Node, the diagram webview runs in a browser
// and draws its SVG itself, so nothing is loaded from the network.
const builds = [
  {
    entryPoints: ["src/extension.ts"],
    bundle: true,
    outfile: "dist/extension.js",
    external: ["vscode"],
    format: "cjs",
    platform: "node",
    target: "node18",
    sourcemap: !production,
    minify: production,
    logLevel: "info",
  },
  {
    entryPoints: ["src/webview/diagram.ts"],
    bundle: true,
    outfile: "dist/webview.js",
    format: "iife",
    platform: "browser",
    target: "es2020",
    sourcemap: !production,
    minify: production,
    logLevel: "info",
  },
];

const contexts = await Promise.all(builds.map((options) => context(options)));

if (watch) {
  await Promise.all(contexts.map((ctx) => ctx.watch()));
} else {
  for (const ctx of contexts) {
    await ctx.rebuild();
    await ctx.dispose();
  }
}
