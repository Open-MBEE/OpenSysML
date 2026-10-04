import { readFile } from "node:fs/promises";
import { isAbsolute, resolve } from "node:path";
import { pathToFileURL } from "node:url";
import { Worker } from "node:worker_threads";
import { Code, ConnectError } from "@connectrpc/connect";
import { Connection } from "../core/connection.js";
import type { TransportOptions } from "../core/connection.js";
import {
  connectWasmHost,
  instantiateInline,
  loadGoConstructor,
  type WasmHost,
  type WasmWorkerRequest,
  type WasmWorkerResponse,
  type WorkerWasmSource,
} from "../core/wasm.js";

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
    const Go = await loadGoConstructor(wasmExec, true);
    host = await instantiateInline(module, Go);
  } else {
    host = await startWorker(options.wasm, wasmExec);
  }
  return connectWasmHost(host, options);
}

async function startWorker(wasm: WasmConnectOptions["wasm"], wasmExec: string): Promise<WasmHost> {
  const worker = new Worker(new URL("./wasm-worker.js", import.meta.url));
  const host = new NodeWasmWorkerHost(worker);
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

class NodeWasmWorkerHost implements WasmHost {
  version = "";

  private readonly pending = new Map<
    number,
    { resolve: (envelope: string) => void; reject: (error: unknown) => void }
  >();
  private readonly ready: Promise<void>;
  private resolveReady!: () => void;
  private rejectReady!: (error: unknown) => void;
  private nextId = 1;
  private failure: ConnectError | undefined;
  private closed = false;

  constructor(private readonly worker: Worker) {
    this.ready = new Promise<void>((resolve, reject) => {
      this.resolveReady = resolve;
      this.rejectReady = reject;
    });
    worker.on("message", (message: WasmWorkerResponse) => {
      this.receive(message);
    });
    worker.on("error", (error: Error) => {
      this.fail(error);
    });
    worker.on("exit", (code: number) => {
      if (!this.closed) {
        this.fail(new Error(`the sysml-wasm worker exited with code ${code}`));
      }
    });
  }

  async start(
    request: Extract<WasmWorkerRequest, { type: "init" }>,
    transfer: readonly ArrayBuffer[],
  ): Promise<this> {
    this.worker.postMessage(request, transfer);
    await this.ready;
    return this;
  }

  async call(method: string, params: string): Promise<string> {
    this.throwIfFailed();
    if (this.closed) {
      throw new ConnectError("the sysml-wasm worker is closed", Code.Unavailable);
    }
    await this.ready;
    this.throwIfFailed();
    const id = this.nextId++;
    return new Promise<string>((resolve, reject) => {
      this.pending.set(id, { resolve, reject });
      try {
        this.worker.postMessage({ type: "call", id, method, params });
      } catch (error) {
        this.pending.delete(id);
        reject(ConnectError.from(error, Code.Unavailable));
      }
    });
  }

  async close(): Promise<void> {
    if (this.closed) {
      return;
    }
    this.closed = true;
    try {
      this.worker.postMessage({ type: "close" });
    } catch {
      // A worker that has already failed has no message loop to close.
    }
    const closed = new ConnectError("the sysml-wasm worker was closed", Code.Unavailable);
    for (const pending of this.pending.values()) {
      pending.reject(closed);
    }
    this.pending.clear();
    await this.worker.terminate();
  }

  private receive(message: WasmWorkerResponse): void {
    if (message.type === "ready") {
      this.version = message.version;
      this.resolveReady();
      return;
    }
    if (message.type === "failed") {
      this.fail(new Error(message.message));
      return;
    }
    const pending = this.pending.get(message.id);
    if (pending !== undefined) {
      this.pending.delete(message.id);
      pending.resolve(message.envelope);
    }
  }

  private fail(reason: unknown): void {
    if (this.failure !== undefined || this.closed) {
      return;
    }
    this.failure = ConnectError.from(reason, Code.Unavailable);
    this.rejectReady(this.failure);
    for (const pending of this.pending.values()) {
      pending.reject(this.failure);
    }
    this.pending.clear();
  }

  private throwIfFailed(): void {
    const failure = this.failure;
    if (failure !== undefined) {
      throw failure;
    }
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

async function loadNodeWasm(wasm: WasmConnectOptions["wasm"]): Promise<BufferSource> {
  if (wasm instanceof Uint8Array) {
    return wasm;
  }
  if (wasm instanceof URL) {
    if (wasm.protocol === "file:") {
      return readFile(wasm);
    }
    const response = await fetch(wasm);
    if (!response.ok) {
      throw new Error(`could not fetch the WebAssembly module: ${response.status}`);
    }
    return response.arrayBuffer();
  }
  const url = parseUrl(wasm);
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
  return readFile(wasm);
}

function moduleSpecifier(source: string | URL): string {
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

function parseUrl(source: string): URL | undefined {
  if (/^[A-Za-z]:[\\/]/.test(source)) {
    return undefined;
  }
  try {
    return new URL(source);
  } catch {
    return undefined;
  }
}
