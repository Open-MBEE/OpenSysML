import { Connection } from "../core/connection.js";
import type { TransportOptions } from "../core/connection.js";
import {
  connectWasmHost,
  instantiateInline,
  loadGoConstructor,
  loadWasmSource,
  WorkerWasmHost,
  type WasmHost,
  type WasmWorkerEndpoint,
  type WasmWorkerResponse,
  type WorkerWasmSource,
} from "../core/wasm.js";

/** Options for connecting to a Go WebAssembly module in a browser. */
export interface BrowserWasmConnectOptions extends TransportOptions {
  /** Module URL, response, bytes, or a compiled WebAssembly module. */
  wasm: string | URL | Response | ArrayBuffer | Uint8Array | WebAssembly.Module;
  /** wasm_exec.js from the Go toolchain that built the module. */
  wasmExec?: string | URL;
  /** A worker running the package's browser WASM worker entry point. */
  worker?: Worker;
  version?: string;
  requireCapabilities?: readonly string[];
}

/**
 * Connects to sysml-wasm inline, or in the provided worker.
 */
export async function connectWasm(
  options: BrowserWasmConnectOptions,
): Promise<Connection> {
  const host =
    options.worker === undefined
      ? await startInline(options.wasm, options.wasmExec)
      : await startWorker(options.worker, options.wasm, options.wasmExec);
  return connectWasmHost(host, options);
}

async function startInline(
  wasm: BrowserWasmConnectOptions["wasm"],
  wasmExec: string | URL | undefined,
): Promise<WasmHost> {
  const module = await browserWasmSource(wasm);
  const Go = await loadGoConstructor(wasmExec === undefined ? undefined : resourceUrl(wasmExec));
  return instantiateInline(module, Go);
}

async function startWorker(
  worker: Worker,
  wasm: BrowserWasmConnectOptions["wasm"],
  wasmExec: string | URL | undefined,
): Promise<WasmHost> {
  const host = new WorkerWasmHost(browserWorkerEndpoint(worker));
  try {
    const { source, transfer } = await workerSource(wasm);
    return await host.start(
      {
        type: "init",
        wasm: source,
        ...(wasmExec === undefined ? {} : { wasmExec: resourceUrl(wasmExec) }),
      },
      transfer,
    );
  } catch (error) {
    await host.close();
    throw error;
  }
}

function browserWorkerEndpoint(worker: Worker): WasmWorkerEndpoint {
  return {
    post(message, transfer) {
      worker.postMessage(message, [...transfer]);
    },
    onMessage(listener) {
      worker.addEventListener("message", (event: MessageEvent<WasmWorkerResponse>) => {
        listener(event.data);
      });
    },
    onFailure(listener) {
      worker.addEventListener("error", (event: ErrorEvent) => {
        listener(new Error(event.message || "the sysml-wasm worker failed"));
      });
      worker.addEventListener("messageerror", () => {
        listener(new Error("the sysml-wasm worker sent an unreadable message"));
      });
    },
    terminate() {
      worker.terminate();
    },
  };
}

async function workerSource(
  wasm: BrowserWasmConnectOptions["wasm"],
): Promise<{ source: WorkerWasmSource; transfer: Transferable[] }> {
  if (wasm instanceof WebAssembly.Module) {
    return { source: wasm, transfer: [] };
  }
  if (wasm instanceof Response) {
    if (wasm.bodyUsed) {
      throw new Error("the WebAssembly Response body has already been read");
    }
    const bytes = await wasm.arrayBuffer();
    return { source: bytes, transfer: [bytes] };
  }
  if (wasm instanceof ArrayBuffer) {
    const bytes = wasm.slice(0);
    return { source: bytes, transfer: [bytes] };
  }
  if (wasm instanceof Uint8Array) {
    const bytes = Uint8Array.from(wasm).buffer;
    return { source: bytes, transfer: [bytes] };
  }
  const url = wasm instanceof URL ? wasm.href : resourceUrl(wasm);
  return { source: url, transfer: [] };
}

async function browserWasmSource(
  wasm: BrowserWasmConnectOptions["wasm"],
): Promise<BufferSource | WebAssembly.Module> {
  if (
    wasm instanceof WebAssembly.Module ||
    wasm instanceof ArrayBuffer ||
    wasm instanceof Uint8Array
  ) {
    return wasm;
  }
  if (wasm instanceof Response && wasm.bodyUsed) {
    throw new Error("the WebAssembly Response body has already been read");
  }
  return loadWasmSource(
    wasm instanceof Response ? wasm : wasm instanceof URL ? wasm : resourceUrl(wasm),
  );
}

function resourceUrl(value: string | URL): string {
  if (value instanceof URL) {
    return value.href;
  }
  const base = typeof document === "undefined" ? import.meta.url : document.baseURI;
  return new URL(value, base).href;
}
