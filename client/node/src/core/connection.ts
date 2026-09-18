// A connection: one Connect client against one service, plus the handshake that
// tells the client what that service can do.

import { Code, ConnectError, createClient, type Client, type Transport } from "@connectrpc/connect";
import { ServerInfo } from "./capabilities.js";
import { ClosedConnectionError } from "./errors.js";
import { SysMLService } from "../generated/sysml_pb.js";
import { callRpc, fromHandshakeError } from "./status.js";
import { Model } from "./model.js";
import type { ParseOptions } from "./model.js";

/** Wire encoding of the request and response bodies. Protobuf is the default. */
export type Encoding = "protobuf" | "json";

/** Observability hook: called with every response the service returns. */
export type ResponseTap = (event: { method: string; response: unknown }) => void;

/** Options shared by every way of opening a connection. */
export interface TransportOptions {
  /** `host:port`, `http://host:port` or `https://host:port` of a running service. */
  address?: string;
  /** Body encoding. Protobuf by default; JSON costs ~6x the server CPU on large answers. */
  encoding?: Encoding;
  /** Deadline applied to every call this connection makes. */
  timeoutMs?: number;
  /** Extra headers sent with every call. */
  headers?: Record<string, string>;
  /** Called with each response, for logging, metrics or a conformance runner. */
  onResponse?: ResponseTap;
}

/**
 * What a connection talks to, and how it lets go. A private child releases its
 * reference on close; an external service is only disconnected from.
 */
export interface ConnectionBackend {
  /** Human-readable provenance, used to name the service in an error message. */
  readonly origin: string;
  /** Releases this connection's hold. Never stops a service the client did not start. */
  release(): Promise<void>;
}

/** A connection to a sysml-grpc service. Close it, or use `await using`. */
export class Connection {
  /**
   * The generated Connect client. The ergonomic API covers what this version
   * supports; this is the escape hatch to the RPCs it does not.
   */
  readonly rpc: Client<typeof SysMLService>;
  readonly info: ServerInfo;
  readonly encoding: Encoding;

  private readonly backend: ConnectionBackend;
  private readonly timeoutMs: number | undefined;
  private closed = false;

  private constructor(init: {
    rpc: Client<typeof SysMLService>;
    info: ServerInfo;
    encoding: Encoding;
    backend: ConnectionBackend;
    timeoutMs?: number | undefined;
  }) {
    this.rpc = init.rpc;
    this.info = init.info;
    this.encoding = init.encoding;
    this.backend = init.backend;
    this.timeoutMs = init.timeoutMs;
  }

  /** Opens a connection over `transport` and performs the capability handshake. */
  static async open(init: {
    transport: Transport;
    backend: ConnectionBackend;
    encoding: Encoding;
    timeoutMs?: number | undefined;
  }): Promise<Connection> {
    const rpc = createClient(SysMLService, init.transport);
    const info = await handshake(rpc, init.backend.origin, init.timeoutMs);
    return new Connection({
      rpc,
      info,
      encoding: init.encoding,
      backend: init.backend,
      ...(init.timeoutMs === undefined ? {} : { timeoutMs: init.timeoutMs }),
    });
  }

  /** Whether this connection has been closed. */
  get isClosed(): boolean {
    return this.closed;
  }

  /** Call options every request of this connection carries. */
  callOptions(): { timeoutMs?: number } {
    if (this.closed) {
      throw new ClosedConnectionError();
    }
    return this.timeoutMs === undefined ? {} : { timeoutMs: this.timeoutMs };
  }

  /** Parses a file the service can read, and returns the model it loaded. */
  load(path: string, options: ParseOptions = {}): Promise<Model> {
    return Model.parse(this, { source: { case: "filePath", value: path } }, options);
  }

  /** Parses inline source text, and returns the model it loaded. */
  loads(source: string, options: ParseOptions = {}): Promise<Model> {
    return Model.parse(this, { source: { case: "content", value: source } }, options);
  }

  /**
   * Adopts a model the service already holds, by hash. Lets a hash pass between
   * processes, and answers NOT_FOUND once the service has evicted the model.
   */
  model(hash: string): Model {
    return Model.adopt(this, hash);
  }

  /** Asks the service what it is and what it can do, again. */
  async serverInfo(): Promise<ServerInfo> {
    const response = await callRpc(this.rpc.getServerInfo({}, this.callOptions()));
    return new ServerInfo({
      version: response.version,
      capabilities: response.capabilities,
      answered: true,
      origin: this.backend.origin,
    });
  }

  /**
   * Releases this connection. A private child loses a reference and stops when
   * the last one goes; a service this client did not start keeps running.
   */
  async close(): Promise<void> {
    if (this.closed) {
      return;
    }
    this.closed = true;
    await this.backend.release();
  }

  async [Symbol.asyncDispose](): Promise<void> {
    await this.close();
  }
}

async function handshake(
  rpc: Client<typeof SysMLService>,
  origin: string,
  timeoutMs: number | undefined,
): Promise<ServerInfo> {
  const options = timeoutMs === undefined ? {} : { timeoutMs };
  try {
    const info = await rpc.getServerInfo({}, options);
    return new ServerInfo({
      version: info.version,
      capabilities: info.capabilities,
      answered: true,
      origin,
    });
  } catch (error) {
    const connectError = ConnectError.from(error);
    // A service too old to answer the handshake is still usable; anything else is not.
    if (connectError.code === Code.Unimplemented) {
      return new ServerInfo({ version: "", capabilities: [], answered: false, origin });
    }
    throw fromHandshakeError(connectError, origin);
  }
}
