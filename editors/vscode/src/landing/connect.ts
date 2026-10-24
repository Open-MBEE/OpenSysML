// Connects the page to the combined sysml-wasm module through the browser client,
// running it in a worker so the diagram stays responsive while it works.

import { connectWasm, type Connection } from "@openmbee/opensysml/browser";

export interface ModuleUrls {
  /** The sysml-wasm module, `.wasm` or `.wasm.gz`. */
  wasm: string;
  /** The wasm_exec.js of the Go toolchain that built the module. */
  wasmExec: string;
  /** The bundled worker entry that hosts the module off the UI thread. */
  worker: string;
}

/** Resolves the page's `data-osml-*` asset URLs against `base`; the worker entry sits beside the landing script. */
export function moduleUrls(dataset: { osmlWasm?: string; osmlWasmExec?: string; osmlScript?: string }, base: string): ModuleUrls {
  const script = new URL(dataset.osmlScript ?? "", base);
  return {
    wasm: new URL(dataset.osmlWasm ?? "", base).href,
    wasmExec: new URL(dataset.osmlWasmExec ?? "", base).href,
    worker: new URL("wasm-worker.js", script).href,
  };
}

/** A `.wasm.gz` asset is inflated here; a plain `.wasm` (a CDN compresses it in transit) is used as the browser hands it over. */
export async function fetchModule(url: string): Promise<ArrayBuffer> {
  const gzipped = /\.gz(?:[?#]|$)/.test(url);
  if (gzipped && typeof DecompressionStream === "undefined") {
    throw new Error("this browser cannot inflate the engine");
  }
  const response = await fetch(url);
  if (!response.ok || response.body === null) {
    throw new Error(`the engine did not download (HTTP ${response.status})`);
  }
  if (!gzipped) {
    return response.arrayBuffer();
  }
  return new Response(response.body.pipeThrough(new DecompressionStream("gzip"))).arrayBuffer();
}

/** Starts the module in a worker and opens a typed connection to it. */
export async function connectModule(urls: ModuleUrls): Promise<Connection> {
  const wasm = await fetchModule(urls.wasm);
  const worker = new Worker(urls.worker, { type: "module" });
  return connectWasm({ wasm, wasmExec: urls.wasmExec, worker });
}
