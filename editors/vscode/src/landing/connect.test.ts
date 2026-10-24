import assert from "node:assert/strict";
import { test } from "node:test";

import { moduleUrls } from "./connect";

test("moduleUrls resolves the page's relative asset URLs and places the worker beside the landing script", () => {
  const urls = moduleUrls(
    {
      osmlWasm: "./assets/sysml-wasm.wasm.gz",
      osmlWasmExec: "./assets/wasm_exec.js",
      osmlScript: "./assets/landing/diagram.js",
    },
    "https://docs.example.org/redk/",
  );
  assert.deepEqual(urls, {
    wasm: "https://docs.example.org/redk/assets/sysml-wasm.wasm.gz",
    wasmExec: "https://docs.example.org/redk/assets/wasm_exec.js",
    worker: "https://docs.example.org/redk/assets/landing/wasm-worker.js",
  });
});

test("moduleUrls keeps an absolute module URL, so the site can point at a CDN package", () => {
  const urls = moduleUrls(
    {
      osmlWasm: "https://cdn.jsdelivr.net/npm/@openmbee/opensysml-wasm@1.0.0/sysml-wasm.wasm",
      osmlWasmExec: "https://cdn.jsdelivr.net/npm/@openmbee/opensysml-wasm@1.0.0/wasm_exec.js",
      osmlScript: "/assets/landing/diagram.js",
    },
    "https://docs.example.org/",
  );
  assert.equal(urls.wasm, "https://cdn.jsdelivr.net/npm/@openmbee/opensysml-wasm@1.0.0/sysml-wasm.wasm");
  assert.equal(urls.wasmExec, "https://cdn.jsdelivr.net/npm/@openmbee/opensysml-wasm@1.0.0/wasm_exec.js");
  assert.equal(urls.worker, "https://docs.example.org/assets/landing/wasm-worker.js");
});
