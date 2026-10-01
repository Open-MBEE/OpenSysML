// Node entry point: everything the core does, plus the private-child lifecycle.

import { writeFile } from "node:fs/promises";
import { createConnectTransport, createGrpcTransport } from "@connectrpc/connect-node";
import type { Transport } from "@connectrpc/connect";
import { Connection } from "../core/connection.js";
import { Conversion, formatOfPath } from "../core/conversion.js";
import type { ConnectionBackend, TransportOptions } from "../core/connection.js";
import { OpenSysMLError } from "../core/errors.js";
import { Model } from "../core/model.js";
import type { ParseOptions } from "../core/model.js";
import { baseUrl, encodingOf, interceptors, timeoutOf } from "../core/transport.js";
import { resolveLatestVersion, VERSION_ENV } from "./binary.js";
import { acquirePrivateService } from "./service.js";

export * from "../core/index.js";
export {
  ALLOW_UNPINNED_ENV,
  API_BASE_URL,
  BINARY_ENV,
  BinaryNotFoundError,
  DEFAULT_GITHUB_REPO,
  NETWORK_TIMEOUT_MS,
  PINNED_SHA256,
  RELEASES_BASE_URL,
  REPO_ENV,
  VERSION_ENV,
  binaryName,
  cachedBinaryPath,
  cachedRelease,
  defaultGithubRepo,
  downloadBinary,
  expectedDigest,
  metadataPath,
  pinnedDigest,
  platformPackage,
  releaseAssetName,
  releaseDownloadUrl,
  resolveBinary,
  resolveLatestVersion,
  signedManifestDigest,
  staleCacheReason,
  unpinnedDownloadsAllowed,
  verifyChecksum,
  writeMetadata,
} from "./binary.js";
export type { Binary, CacheMetadata, DownloadOptions, PinnedDigests } from "./binary.js";
export {
  BUNDLE_ASSET,
  MANIFEST_ASSET,
  ReleaseSigner,
  SIGNED_MANIFEST_SIGNERS,
  manifestDigest,
  signerFor,
  verifiedManifestDigest,
  verifyManifest,
} from "./signing.js";
export { PrivateService, currentPrivateService } from "./service.js";

/** Names a service to connect to instead of starting one. */
export const SERVICE_ENV = "OPENSYSML_SERVICE";

/** How this process connects. Without an address it starts a private child. */
export interface ConnectOptions extends TransportOptions {
  /** HTTP version used to reach the service. HTTP/2 (h2c over plain http) by default. */
  httpVersion?: "1.1" | "2";
  /** Wire protocol. Connect by default; gRPC needs HTTP/2 and a service that serves it. */
  protocol?: "connect" | "grpc";
  /**
   * Release tag the service must report, or 'latest'; $OPENSYSML_GRPC_VERSION
   * when omitted. A mismatch refuses the connection with a StaleServiceError.
   */
  version?: string;
  /** Capabilities the service must report for the connection to be returned. */
  requireCapabilities?: readonly string[];
}

/**
 * Connects to a sysml-grpc service.
 *
 * With no address, this starts a private child of this process — one per thread,
 * shared by every connection, stopped when the last one closes. With an address,
 * or with `$OPENSYSML_SERVICE` set, it connects to a service someone else runs and
 * closing the connection leaves that service running.
 */
export async function connect(options: ConnectOptions = {}): Promise<Connection> {
  const address = options.address ?? process.env[SERVICE_ENV];
  if (address !== undefined && address !== "") {
    return connectExternal(address, options);
  }
  return connectPrivate(options);
}

/**
 * Writes `target` to `path`: a model converted in the format its extension
 * names, or a conversion's or edit result's content written as the service
 * returned it (empty for a multi-document edit).
 */
export async function save(
  target: Model | Conversion,
  path: string,
  options: { toFormat?: string; tolerateSyntaxErrors?: boolean } = {},
): Promise<Conversion> {
  const conversion =
    target instanceof Conversion
      ? target
      : await target.convert(options.toFormat ?? formatOfPath(path), {
          ...(options.tolerateSyntaxErrors === undefined
            ? {}
            : { tolerateSyntaxErrors: options.tolerateSyntaxErrors }),
        });
  await writeFile(path, conversion.content, "utf8");
  return conversion;
}

/** Parses a file over a connection of its own, which the model closes. */
export async function load(path: string, options: ConnectOptions & ParseOptions = {}): Promise<Model> {
  const connection = await connect(options);
  try {
    return await Model.parse(connection, { source: { case: "filePath", value: path } }, options, true);
  } catch (error) {
    await connection.close();
    throw error;
  }
}

/** Parses inline source over a connection of its own, which the model closes. */
export async function loads(source: string, options: ConnectOptions & ParseOptions = {}): Promise<Model> {
  const connection = await connect(options);
  try {
    return await Model.parse(connection, { source: { case: "content", value: source } }, options, true);
  } catch (error) {
    await connection.close();
    throw error;
  }
}

// The release a connection requires, 'latest' resolved to the tag it names.
async function requiredRelease(options: ConnectOptions): Promise<string | undefined> {
  const asked = options.version ?? process.env[VERSION_ENV];
  if (asked === undefined || asked === "") {
    return undefined;
  }
  if (asked === "latest") {
    try {
      return await resolveLatestVersion();
    } catch {
      // A release that cannot be resolved is not required, as ensure_binary reads it.
      return undefined;
    }
  }
  return asked;
}

async function connectPrivate(options: ConnectOptions): Promise<Connection> {
  // Checked before a child is started, so a bad option costs no process.
  const encoding = encodingOf(options);
  const timeoutMs = timeoutOf(options);
  const required = await requiredRelease(options);
  const service = await acquirePrivateService(required);
  const backend: ConnectionBackend = {
    origin: `${service.binary.path}, started by this client`,
    release: () => service.release(),
    warn: (message, type) => {
      process.emitWarning(message, type);
    },
  };
  try {
    return await Connection.open({
      transport: transportFor(baseUrl(service.address), options),
      encoding,
      backend,
      timeoutMs,
      ...(required === undefined ? {} : { requiredVersion: required }),
      ...(options.requireCapabilities === undefined
        ? {}
        : { requiredCapabilities: options.requireCapabilities }),
      stale: {
        address: service.address,
        remedy:
          `the binary this client started, ${service.binary.path}, is not ` +
          `${required ?? "the release asked for"}: make that release available (its ` +
          `download is cached under ~/.opensysml/bin), or accept what is installed ` +
          `by passing version: undefined and unsetting $${VERSION_ENV}`,
      },
    });
  } catch (error) {
    await service.release();
    throw error;
  }
}

async function connectExternal(address: string, options: ConnectOptions): Promise<Connection> {
  const url = baseUrl(address);
  const timeoutMs = timeoutOf(options);
  const required = await requiredRelease(options);
  return Connection.open({
    transport: transportFor(url, options),
    encoding: encodingOf(options),
    backend: {
      origin: `${url}, which this client did not start`,
      // A service this client did not start is never stopped by it.
      release: () => Promise.resolve(),
      warn: (message, type) => {
        process.emitWarning(message, type);
      },
    },
    timeoutMs,
    ...(required === undefined ? {} : { requiredVersion: required }),
    ...(options.requireCapabilities === undefined
      ? {}
      : { requiredCapabilities: options.requireCapabilities }),
    stale: {
      address: url,
      remedy:
        `stop the service listening on ${url} yourself and let this client start ` +
        `a ${required ?? "matching"} one, or accept what is running by passing ` +
        `version: undefined and unsetting $${VERSION_ENV}`,
    },
  });
}

function transportFor(url: string, options: ConnectOptions): Transport {
  const binary = encodingOf(options) === "protobuf";
  if (options.protocol === "grpc") {
    if (!binary) {
      throw new OpenSysMLError("the gRPC protocol carries protobuf bodies; JSON needs the Connect protocol");
    }
    return createGrpcTransport({ baseUrl: url, interceptors: interceptors(options) });
  }
  return createConnectTransport({
    baseUrl: url,
    httpVersion: options.httpVersion ?? "2",
    useBinaryFormat: binary,
    interceptors: interceptors(options),
  });
}
