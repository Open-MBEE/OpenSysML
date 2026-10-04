import { readFile } from "node:fs/promises";
import { isAbsolute, resolve } from "node:path";
import { pathToFileURL } from "node:url";
import type { WorkerWasmSource } from "../core/wasm.js";

export async function loadNodeWasm(
  source: WorkerWasmSource | URL,
): Promise<BufferSource | WebAssembly.Module> {
  if (
    source instanceof WebAssembly.Module ||
    source instanceof ArrayBuffer ||
    ArrayBuffer.isView(source)
  ) {
    return source;
  }
  const url = source instanceof URL ? source : parseUrl(source);
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
  if (typeof source === "string") {
    return readFile(isAbsolute(source) ? source : resolve(source));
  }
  return readFile(source);
}

export function moduleSpecifier(source: string | URL): string {
  if (source instanceof URL) {
    return source.href;
  }
  if (!/^[A-Za-z]:[\\/]/.test(source)) {
    try {
      return new URL(source).href;
    } catch {
      // Resolve filesystem paths relative to the current working directory.
    }
  }
  return pathToFileURL(isAbsolute(source) ? source : resolve(source)).href;
}

export function parseUrl(source: string): URL | undefined {
  if (/^[A-Za-z]:[\\/]/.test(source)) {
    return undefined;
  }
  try {
    return new URL(source);
  } catch {
    return undefined;
  }
}
