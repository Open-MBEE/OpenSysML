import { readFile } from "node:fs/promises";
import { resolve } from "node:path";
import { pathToFileURL } from "node:url";
import { parentPort } from "node:worker_threads";
import {
  loadGoConstructor,
  serveWasmPort,
  type WasmPortLike,
  type WorkerWasmSource,
} from "../core/wasm.js";

if (parentPort === null) {
  throw new Error("the sysml-wasm worker must run in a worker thread");
}

serveWasmPort(parentPort as unknown as WasmPortLike, {
  loadWasm: loadNodeWasm,
  loadGo: (wasmExec) => loadGoConstructor(wasmExec === undefined ? undefined : moduleSpecifier(wasmExec)),
});

async function loadNodeWasm(source: WorkerWasmSource): Promise<BufferSource | WebAssembly.Module> {
  if (
    source instanceof WebAssembly.Module ||
    source instanceof ArrayBuffer ||
    ArrayBuffer.isView(source)
  ) {
    return source;
  }
  const url = parseUrl(source);
  if (url?.protocol === "file:") {
    return readFile(url);
  }
  if (url !== undefined) {
    const response = await fetch(url);
    if (!response.ok) {
      throw new Error(`could not fetch the WebAssembly module: ${response.status}`);
    }
    return response.arrayBuffer();
  }
  return readFile(resolve(source));
}

function moduleSpecifier(source: string): string {
  return parseUrl(source)?.href ?? pathToFileURL(resolve(source)).href;
}

function parseUrl(value: string): URL | undefined {
  if (/^[A-Za-z]:[\\/]/.test(value)) {
    return undefined;
  }
  try {
    return new URL(value);
  } catch {
    return undefined;
  }
}
