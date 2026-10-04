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
  const host = new BrowserWasmWorkerHost(worker);
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

class BrowserWasmWorkerHost implements WasmHost {
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
    worker.addEventListener("message", (event: MessageEvent<WasmWorkerResponse>) => {
      this.receive(event.data);
    });
    worker.addEventListener("error", (event: ErrorEvent) => {
      this.fail(new Error(event.message || "the sysml-wasm worker failed"));
    });
    worker.addEventListener("messageerror", () => {
      this.fail(new Error("the sysml-wasm worker sent an unreadable message"));
    });
  }

  async start(
    request: Extract<WasmWorkerRequest, { type: "init" }>,
    transfer: Transferable[],
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

  close(): Promise<void> {
    if (this.closed) {
      return Promise.resolve();
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
    this.worker.terminate();
    return Promise.resolve();
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
  const response =
    wasm instanceof Response ? wasm : await fetch(wasm instanceof URL ? wasm : resourceUrl(wasm));
  if (!response.ok) {
    throw new Error(`could not fetch the WebAssembly module: ${response.status}`);
  }
  if (response.bodyUsed) {
    throw new Error("the WebAssembly Response body has already been read");
  }
  if (typeof WebAssembly.compileStreaming === "function") {
    try {
      return await WebAssembly.compileStreaming(response.clone());
    } catch {
      // Some servers omit the application/wasm content type required by streaming.
    }
  }
  return response.arrayBuffer();
}

function resourceUrl(value: string | URL): string {
  if (value instanceof URL) {
    return value.href;
  }
  const base = typeof document === "undefined" ? import.meta.url : document.baseURI;
  return new URL(value, base).href;
}
