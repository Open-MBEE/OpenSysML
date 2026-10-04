import { existsSync, readFileSync } from "node:fs";
import { readFile } from "node:fs/promises";
import { createRequire } from "node:module";
import { dirname, isAbsolute, join, resolve } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";
import { OpenSysMLError } from "../core/errors.js";
import { PACKAGE_NAME, WASM_PACKAGE } from "../core/package.js";
import type { WorkerWasmSource } from "../core/wasm.js";

export interface NodeWasmSourceOptions {
  wasm?: string | URL | Uint8Array;
  wasmExec?: string | URL;
}

export interface ResolvedNodeWasmSources {
  wasm: string | URL | Uint8Array;
  wasmExec: string | URL;
}

export type PackageJsonResolver = (specifier: string) => string;

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

export function resolveWasmSources(
  options: NodeWasmSourceOptions,
  resolvePackageJson: PackageJsonResolver = createRequire(import.meta.url).resolve,
): ResolvedNodeWasmSources {
  if (options.wasm !== undefined) {
    if (options.wasmExec === undefined) {
      throw new OpenSysMLError("connectWasm needs the wasmExec option when wasm is provided");
    }
    return { wasm: options.wasm, wasmExec: options.wasmExec };
  }

  const assets = resolveWasmPackage(resolvePackageJson);
  return {
    wasm: assets.wasm,
    wasmExec: options.wasmExec ?? assets.wasmExec,
  };
}

export function resolveWasmPackage(
  resolvePackageJson: PackageJsonResolver = createRequire(import.meta.url).resolve,
): { wasm: string; wasmExec: string } {
  let packageJson: string;
  try {
    packageJson = resolvePackageJson(`${WASM_PACKAGE}/package.json`);
  } catch (cause) {
    throw new OpenSysMLError(
      `connectWasm needs sysml-wasm.wasm: install ${WASM_PACKAGE} at ${clientVersion()}, or pass wasm and wasmExec`,
      { cause },
    );
  }
  const packageDirectory = dirname(packageJson);
  return {
    wasm: join(packageDirectory, "sysml-wasm.wasm"),
    wasmExec: join(packageDirectory, "wasm_exec.js"),
  };
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

function clientVersion(): string {
  let directory = dirname(fileURLToPath(import.meta.url));
  for (;;) {
    const packageJson = join(directory, "package.json");
    if (existsSync(packageJson)) {
      const metadata: unknown = JSON.parse(readFileSync(packageJson, "utf8"));
      if (
        typeof metadata === "object" &&
        metadata !== null &&
        "name" in metadata &&
        metadata.name === PACKAGE_NAME &&
        "version" in metadata &&
        typeof metadata.version === "string"
      ) {
        return metadata.version;
      }
    }
    const parent = dirname(directory);
    if (parent === directory) {
      throw new OpenSysMLError("the OpenSysML Node package version could not be found");
    }
    directory = parent;
  }
}
