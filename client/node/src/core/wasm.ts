import {
  fromJson,
  toJson,
  type DescMessage,
  type DescMethodUnary,
  type JsonValue,
} from "@bufbuild/protobuf";
import {
  Code,
  ConnectError,
  createContextValues,
  type StreamResponse,
  type Transport,
  type UnaryResponse,
} from "@connectrpc/connect";
import { getAbortSignalReason, runUnaryCall } from "@connectrpc/connect/protocol";
import { Connection } from "./connection.js";
import type { TransportOptions } from "./connection.js";
import { ClosedConnectionError, OpenSysMLError } from "./errors.js";
import { encodingOf, interceptors, timeoutOf } from "./transport.js";

/** A host surface implemented by an inline Go runtime or a worker. */
export interface WasmHost {
  readonly version: string;
  call(method: string, params: string): Promise<string>;
  close(): Promise<void>;
}

/** The Go runtime interface exposed by wasm_exec.js. */
export interface GoRuntime {
  readonly importObject: WebAssembly.Imports;
  run(instance: WebAssembly.Instance): Promise<unknown>;
}

/** A constructor exported globally by wasm_exec.js. */
export type GoConstructor = new () => GoRuntime;

export type WorkerWasmSource = string | ArrayBuffer | Uint8Array | WebAssembly.Module;

export type WasmWorkerRequest =
  | { type: "init"; wasm: WorkerWasmSource; wasmExec?: string }
  | { type: "call"; id: number; method: string; params: string }
  | { type: "close" };

export type WasmWorkerResponse =
  | { type: "ready"; version: string }
  | { type: "failed"; message: string }
  | { type: "answer"; id: number; envelope: string };

/** The message port surface shared by browser and Node workers. */
export interface WasmPortLike {
  postMessage(message: WasmWorkerResponse): void;
  addEventListener(
    type: "message",
    listener: (event: MessageEvent<WasmWorkerRequest>) => void,
  ): void;
  start?(): void;
}

/** Optional platform loaders for worker runtimes that need local file access. */
export interface WasmPortLoaders {
  loadWasm?(source: WorkerWasmSource): Promise<BufferSource | WebAssembly.Module>;
  loadGo?(wasmExec: string | undefined): Promise<GoConstructor>;
}

/** Connection settings shared by the Node and browser WASM clients. */
export interface WasmConnectionOptions extends TransportOptions {
  version?: string;
  requireCapabilities?: readonly string[];
}

interface GlobalWasmSurface {
  readonly version: string;
  call(method: string, params: string): string;
}

let inlineStartup = Promise.resolve();

/**
 * Adapts the Go host global to the asynchronous host interface.
 */
export function wasmHostFromGlobal(surface?: GlobalWasmSurface): WasmHost {
  const global = globalThis as typeof globalThis & { sysmlWasm?: GlobalWasmSurface };
  const hostSurface = surface ?? global.sysmlWasm;
  if (
    hostSurface === undefined ||
    typeof hostSurface.version !== "string" ||
    typeof hostSurface.call !== "function"
  ) {
    throw new OpenSysMLError("the Go runtime did not expose globalThis.sysmlWasm");
  }
  let closed = false;
  return {
    version: hostSurface.version,
    call(method, params) {
      if (closed) {
        return Promise.reject(new ClosedConnectionError());
      }
      try {
        return Promise.resolve(hostSurface.call(method, params));
      } catch (cause) {
        return Promise.resolve(errorEnvelope(cause));
      }
    },
    close() {
      closed = true;
      return Promise.resolve();
    },
  };
}

/**
 * Starts a Go WebAssembly module in this realm and captures its host global.
 */
export async function instantiateInline(
  module: BufferSource | WebAssembly.Module,
  Go: GoConstructor,
): Promise<WasmHost> {
  const starting = inlineStartup.then(() => instantiateInlineNow(module, Go));
  inlineStartup = starting.then(
    () => undefined,
    () => undefined,
  );
  return await starting;
}

async function instantiateInlineNow(
  module: BufferSource | WebAssembly.Module,
  Go: GoConstructor,
): Promise<WasmHost> {
  const runtime = globalThis as typeof globalThis & { sysmlWasm?: GlobalWasmSurface };
  delete runtime.sysmlWasm;

  const go = new Go();
  const instance =
    module instanceof WebAssembly.Module
      ? await WebAssembly.instantiate(module, go.importObject)
      : (await WebAssembly.instantiate(module, go.importObject)).instance;
  let startError: unknown;
  void go.run(instance).catch((error: unknown) => {
    startError = error;
  });
  await new Promise<void>((resolve) => setTimeout(resolve, 0));

  const surface = globalWasmSurface();
  delete runtime.sysmlWasm;
  if (surface === undefined) {
    throw new OpenSysMLError(
      "the Go runtime did not expose globalThis.sysmlWasm",
      startError === undefined ? undefined : { cause: startError },
    );
  }
  return wasmHostFromGlobal(surface);
}

/**
 * Creates a Connect transport for the JSON envelope served by sysml-wasm.
 */
export function createWasmTransport(host: WasmHost, options: TransportOptions): Transport {
  const callUnary = <I extends DescMessage, O extends DescMessage>(
    method: DescMethodUnary<I, O>,
    signal: AbortSignal | undefined,
    timeoutMs: number | undefined,
    header: HeadersInit | undefined,
    input: Parameters<Transport["unary"]>[4],
    contextValues: Parameters<Transport["unary"]>[5],
  ): Promise<UnaryResponse<I, O>> =>
    runUnaryCall({
      req: {
        stream: false,
        service: method.parent,
        method,
        requestMethod: "POST",
        url: `wasm://sysml-wasm/${method.parent.typeName}/${method.name}`,
        header: new Headers(header),
        contextValues: contextValues ?? createContextValues(),
        message: input as never,
      },
      ...(signal === undefined ? {} : { signal }),
      ...(timeoutMs === undefined ? {} : { timeoutMs }),
      interceptors: interceptors(options),
      next: async (request) => {
        if (request.signal.aborted) {
          throw abortError(request.signal);
        }
        const params = JSON.stringify(toJson(method.input, request.message));
        const answer = host.call(method.name, params);
        const envelope = await withAbort(answer, request.signal);
        const body = readEnvelope(envelope);
        return {
          stream: false,
          service: method.parent,
          method,
          header: new Headers(),
          message: fromJson(method.output, body as JsonValue, {
            ignoreUnknownFields: true,
          }),
          trailer: new Headers(),
        };
      },
    });

  return {
    unary: callUnary,
    stream<I extends DescMessage, O extends DescMessage>(): Promise<StreamResponse<I, O>> {
      return Promise.reject(
        new ConnectError("streaming methods are not supported by sysml-wasm", Code.Unimplemented),
      );
    },
  };
}

/**
 * Opens a Connection over a WASM host, negotiating its version and capabilities.
 */
export async function connectWasmHost(
  host: WasmHost,
  options: WasmConnectionOptions = {},
): Promise<Connection> {
  let timeoutMs: number | undefined;
  try {
    if (options.encoding !== undefined && encodingOf(options) !== "json") {
      throw new OpenSysMLError("sysml-wasm supports JSON encoding only");
    }
    timeoutMs = timeoutOf(options);
  } catch (error) {
    await host.close().catch(() => undefined);
    throw error;
  }
  const required =
    options.version === undefined || options.version === "" || options.version === "latest"
      ? undefined
      : options.version;
  const origin = `sysml-wasm ${host.version}`;
  return Connection.open({
    transport: createWasmTransport(host, options),
    backend: {
      origin,
      release: () => host.close(),
      warn: (message) => {
        console.warn(message);
      },
    },
    encoding: "json",
    timeoutMs,
    ...(required === undefined ? {} : { requiredVersion: required }),
    ...(options.requireCapabilities === undefined
      ? {}
      : { requiredCapabilities: options.requireCapabilities }),
    stale: {
      address: origin,
      remedy: required === undefined
        ? "use a module reporting the expected version, or omit version"
        : `use a module reporting ${required}, or omit version`,
    },
  });
}

/**
 * Serves the shared init/call protocol on a Node or browser worker port.
 */
export function serveWasmPort(port: WasmPortLike, loaders: WasmPortLoaders = {}): void {
  let host: WasmHost | undefined;
  let initializing = false;
  let closed = false;
  let calls = Promise.resolve();

  const onMessage = (event: MessageEvent<WasmWorkerRequest>): void => {
    const request = event.data;
    if (request.type === "init") {
      if (initializing || host !== undefined || closed) {
        port.postMessage({ type: "failed", message: "the WASM worker is already initialized" });
        return;
      }
      initializing = true;
      void initialize(request.wasm, request.wasmExec);
      return;
    }
    if (request.type === "close") {
      closed = true;
      void host?.close();
      return;
    }
    calls = calls.then(async () => {
      if (closed || host === undefined) {
        port.postMessage({
          type: "answer",
          id: request.id,
          envelope: errorEnvelope(new ConnectError("the WASM worker is unavailable", Code.Unavailable)),
        });
        return;
      }
      try {
        port.postMessage({
          type: "answer",
          id: request.id,
          envelope: await host.call(request.method, request.params),
        });
      } catch (cause) {
        port.postMessage({ type: "answer", id: request.id, envelope: errorEnvelope(cause) });
      }
    });
  };

  const initialize = async (
    source: WorkerWasmSource,
    wasmExec: string | undefined,
  ): Promise<void> => {
    try {
      const module = loaders.loadWasm
        ? await loaders.loadWasm(source)
        : await loadWasmSource(source);
      const Go = loaders.loadGo
        ? await loaders.loadGo(wasmExec)
        : await loadGoConstructor(wasmExec);
      host = await instantiateInline(module, Go);
      port.postMessage({ type: "ready", version: host.version });
    } catch (cause) {
      port.postMessage({ type: "failed", message: errorMessage(cause) });
    }
  };

  port.addEventListener("message", onMessage);
  port.start?.();
}

/** Loads the Go constructor installed by wasm_exec.js, importing it when needed. */
export async function loadGoConstructor(
  wasmExec?: string,
  forceImport = false,
): Promise<GoConstructor> {
  const runtime = globalThis as typeof globalThis & { Go?: GoConstructor };
  if (wasmExec !== undefined && (forceImport || runtime.Go === undefined)) {
    try {
      await import(wasmExec);
    } catch (cause) {
      const importScripts = (
        globalThis as typeof globalThis & { importScripts?: (...urls: string[]) => void }
      ).importScripts;
      if (importScripts === undefined) {
        throw new OpenSysMLError(`could not load wasm_exec.js from ${wasmExec}`, { cause });
      }
      importScripts(wasmExec);
    }
  }
  if (runtime.Go === undefined) {
    throw new OpenSysMLError("Go is unavailable; supply the matching wasm_exec.js");
  }
  return runtime.Go;
}

function readEnvelope(envelope: string): unknown {
  let parsed: unknown;
  try {
    parsed = JSON.parse(envelope) as unknown;
  } catch (cause) {
    throw ConnectError.from(cause, Code.Internal);
  }
  if (parsed === null || typeof parsed !== "object") {
    throw new ConnectError("sysml-wasm returned an invalid JSON envelope", Code.Internal);
  }
  const response = parsed as {
    result?: unknown;
    error?: { code?: unknown; message?: unknown } | null;
  };
  if (response.error !== undefined && response.error !== null) {
    const status =
      typeof response.error.code === "number" &&
      Object.values(Code).includes(response.error.code)
        ? response.error.code
        : Code.Unknown;
    throw new ConnectError(
      typeof response.error.message === "string"
        ? response.error.message
        : "sysml-wasm returned an error",
      status,
    );
  }
  if (!Object.hasOwn(response, "result")) {
    throw new ConnectError("sysml-wasm returned an envelope without a result", Code.Internal);
  }
  return response.result;
}

function withAbort<T>(answer: Promise<T>, signal: AbortSignal): Promise<T> {
  if (signal.aborted) {
    return Promise.reject(abortError(signal));
  }
  return new Promise<T>((resolve, reject) => {
    let settled = false;
    const cleanup = (): void => {
      signal.removeEventListener("abort", onAbort);
    };
    const finish = (callback: (value: T) => void, value: T): void => {
      if (!settled) {
        settled = true;
        cleanup();
        callback(value);
      }
    };
    const fail = (error: unknown): void => {
      if (!settled) {
        settled = true;
        cleanup();
        reject(ConnectError.from(error, Code.Internal));
      }
    };
    const onAbort = (): void => {
      fail(abortError(signal));
    };
    signal.addEventListener("abort", onAbort, { once: true });
    answer.then(
      (value) => {
        finish(resolve, value);
      },
      fail,
    );
  });
}

function abortError(signal: AbortSignal): ConnectError {
  return ConnectError.from(getAbortSignalReason(signal), Code.Canceled);
}

function errorEnvelope(error: unknown): string {
  const connectError = ConnectError.from(error, Code.Internal);
  return JSON.stringify({
    jsonrpc: "2.0",
    id: null,
    error: { code: connectError.code, message: connectError.rawMessage },
  });
}

async function loadWasmSource(
  source: WorkerWasmSource,
): Promise<BufferSource | WebAssembly.Module> {
  if (
    source instanceof WebAssembly.Module ||
    source instanceof ArrayBuffer ||
    ArrayBuffer.isView(source)
  ) {
    return source;
  }
  const response = await fetch(source);
  if (!response.ok) {
    throw new OpenSysMLError(`could not fetch the WebAssembly module: ${response.status}`);
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

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

function globalWasmSurface(): GlobalWasmSurface | undefined {
  return (globalThis as typeof globalThis & { sysmlWasm?: GlobalWasmSurface }).sysmlWasm;
}
