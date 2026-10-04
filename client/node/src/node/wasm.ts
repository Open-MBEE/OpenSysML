import { Worker } from "node:worker_threads";
import { Connection } from "../core/connection.js";
import type { TransportOptions } from "../core/connection.js";
import {
  connectWasmHost,
  instantiateInline,
  loadGoConstructor,
  WorkerWasmHost,
  type WasmHost,
  type WasmWorkerEndpoint,
  type WorkerWasmSource,
} from "../core/wasm.js";
import { loadNodeWasm, moduleSpecifier } from "./wasm-source.js";

/** Options for connecting to a Go WebAssembly module from Node. */
export interface WasmConnectOptions extends TransportOptions {
  /** Path, file URL, HTTP URL, or bytes of sysml-wasm.wasm. */
  wasm: string | URL | Uint8Array;
  /** wasm_exec.js from the Go toolchain that built the module. */
  wasmExec: string | URL;
  /** Run the module in a worker thread (default) or this thread. */
  thread?: "worker" | "inline";
  version?: string;
  requireCapabilities?: readonly string[];
}

/**
 * Connects to sysml-wasm, in a worker by default or inline when requested.
 */
export async function connectWasm(options: WasmConnectOptions): Promise<Connection> {
  const wasmExec = moduleSpecifier(options.wasmExec);
  let host: WasmHost;
  if (options.thread === "inline") {
    const module = await loadNodeWasm(options.wasm);
    const Go = await loadGoConstructor(wasmExec);
    host = await instantiateInline(module, Go);
  } else {
    host = await startWorker(options.wasm, wasmExec);
  }
  return connectWasmHost(host, options);
}

async function startWorker(wasm: WasmConnectOptions["wasm"], wasmExec: string): Promise<WasmHost> {
  const worker = new Worker(new URL("./wasm-worker.js", import.meta.url));
  const host = new WorkerWasmHost(nodeWorkerEndpoint(worker));
  const { source, transfer } = workerSource(wasm);
  try {
    return await host.start(
      { type: "init", wasm: source, wasmExec },
      transfer,
    );
  } catch (error) {
    await host.close();
    throw error;
  }
}

function workerSource(
  wasm: WasmConnectOptions["wasm"],
): { source: WorkerWasmSource; transfer: readonly ArrayBuffer[] } {
  if (wasm instanceof Uint8Array) {
    const bytes = Uint8Array.from(wasm);
    return { source: bytes.buffer, transfer: [bytes.buffer] };
  }
  return { source: wasm instanceof URL ? wasm.href : wasm, transfer: [] };
}

function nodeWorkerEndpoint(worker: Worker): WasmWorkerEndpoint {
  let closed = false;
  return {
    post(message, transfer) {
      worker.postMessage(message, [...transfer] as ArrayBuffer[]);
    },
    onMessage(listener) {
      worker.on("message", listener);
    },
    onFailure(listener) {
      worker.on("error", listener);
      worker.on("exit", (code: number) => {
        if (!closed) {
          listener(new Error(`the sysml-wasm worker exited with code ${code}`));
        }
      });
    },
    async terminate() {
      closed = true;
      await worker.terminate();
    },
  };
}
