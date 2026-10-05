// Node entry point: everything the core does, plus the private-child lifecycle.

import { constants } from "node:fs";
import {
  lstat,
  mkdir,
  readlink,
  realpath,
  stat,
  writeFile,
} from "node:fs/promises";
import {
  basename,
  dirname,
  isAbsolute,
  join,
  relative,
  resolve,
  sep,
} from "node:path";
import {
  createConnectTransport,
  createGrpcTransport,
} from "@connectrpc/connect-node";
import type { Transport } from "@connectrpc/connect";
import { Connection } from "../core/connection.js";
import { Conversion, Migration, formatOfPath } from "../core/conversion.js";
import type {
  ConnectionBackend,
  TransportOptions,
} from "../core/connection.js";
import { OpenSysMLError } from "../core/errors.js";
import { Model } from "../core/model.js";
import type { ParseOptions } from "../core/model.js";
import {
  baseUrl,
  encodingOf,
  interceptors,
  timeoutOf,
} from "../core/transport.js";
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
export type {
  Binary,
  CacheMetadata,
  DownloadOptions,
  PinnedDigests,
} from "./binary.js";
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
export { connectWasm } from "./wasm.js";
export type { WasmConnectOptions } from "./wasm.js";

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
export async function connect(
  options: ConnectOptions = {},
): Promise<Connection> {
  const address = options.address ?? process.env[SERVICE_ENV];
  if (address !== undefined && address !== "") {
    return connectExternal(address, options);
  }
  return connectPrivate(options);
}

/**
 * Writes `target` to `path`: a model converted in the format its extension
 * names, a conversion's content written as the service returned it, or a
 * migration's content with its image files beside it, at the relative paths
 * the migrated model refers to them with, as `sysml -migrate -o` writes them.
 * A migration that would replace the v1 model it was read from, or land an
 * image outside `path`'s directory, over the model or through a symbolic link
 * at its path, is refused with a `RangeError` before anything is written.
 * Images are then written without following a link at their path, so one put
 * there after the check is not followed; a directory on the way replaced
 * while the write is under way is not guarded against, as `sysml -migrate -o`
 * does not either.
 */
export async function save(target: Migration, path: string): Promise<Migration>;
export async function save(
  target: Model | Conversion,
  path: string,
  options?: { toFormat?: string; tolerateSyntaxErrors?: boolean },
): Promise<Conversion>;
export async function save(
  target: Model | Conversion | Migration,
  path: string,
  options: { toFormat?: string; tolerateSyntaxErrors?: boolean } = {},
): Promise<Conversion | Migration> {
  if (target instanceof Migration) {
    const files = await migrationFiles(target, path);
    await writeFile(path, target.content, "utf8");
    await files.reduce(
      (previous, [file, data]) => previous.then(() => writeImage(file, data)),
      Promise.resolve(),
    );
    return target;
  }
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
/**
 * Where a migration's image files land when its model is written to `path`,
 * refusing any that would escape the model's directory or replace the model
 * or the v1 source, and refusing `path` itself when it is the source. A source
 * no longer at its path protects nothing: nothing of it would be replaced.
 */
async function migrationFiles(
  migration: Migration,
  path: string,
): Promise<[string, Uint8Array][]> {
  const source = (await exists(migration.sourcePath))
    ? migration.sourcePath
    : "";
  if (source !== "" && (await samePath(path, source))) {
    throw new RangeError(
      `${path} names the model being migrated; the v1 model would be replaced by its migration`,
    );
  }
  const base = resolve(dirname(path));
  const landedBase = await landing(base);
  const guarded = [path, source].filter((candidate) => candidate !== "");
  return Promise.all(
    [...migration.files].map(async ([name, data]): Promise<[string, Uint8Array]> => {
      const file = await imageFile(name, base, landedBase, guarded);
      return [file, data];
    }),
  );
}

/**
 * Where the image `name` lands under `base`, refusing one that would escape
 * `base`, replace a guarded path, or be written through a symbolic link.
 */
async function imageFile(
  name: string,
  base: string,
  landedBase: string,
  guarded: readonly string[],
): Promise<string> {
  const segments = name.split("/");
  const file = resolve(base, ...segments);
  if (
    segments.some(
      (segment) =>
        segment === "" ||
        segment === "." ||
        segment === ".." ||
        segment.includes("\\"),
    ) ||
    !within(base, file) ||
    !within(landedBase, await landing(file))
  ) {
    throw new RangeError(
      `the migration's image ${name} would land outside ${base}`,
    );
  }
  const replaced = await Promise.all(guarded.map((candidate) => samePath(file, candidate)));
  const replacing = guarded.find((_, index) => replaced[index]);
  if (replacing !== undefined) {
    throw new RangeError(
      `the migration's image ${name} would replace ${replacing}`,
    );
  }
  if (await isSymbolicLink(file)) {
    throw new RangeError(
      `the migration's image ${name} would be written through a symbolic link at ${file}`,
    );
  }
  return file;
}

/** Writes one image, creating its directory. */
async function writeImage(file: string, data: Uint8Array): Promise<void> {
  await mkdir(dirname(file), { recursive: true });
  await writeFile(file, data, { flag: IMAGE_FLAGS });
}

/**
 * How an image is written: created or truncated in place, never through a link
 * at its path. Windows has no O_NOFOLLOW; the bitwise or reads its absence as 0.
 */
const IMAGE_FLAGS =
  constants.O_WRONLY |
  constants.O_CREAT |
  constants.O_TRUNC |
  constants.O_NOFOLLOW;

/** Whether `path` is itself a symbolic link, wherever it points. */
function isSymbolicLink(path: string): Promise<boolean> {
  return lstat(path).then(
    (status) => status.isSymbolicLink(),
    () => false,
  );
}

/** Whether `path` names something: an empty path, or one that is gone, does not. */
async function exists(path: string): Promise<boolean> {
  return (
    path !== "" &&
    (await stat(path).then(
      () => true,
      () => false,
    ))
  );
}

/** Whether `file` lies strictly below the directory `base`, both absolute. */
function within(base: string, file: string): boolean {
  const below = relative(base, file);
  return (
    below !== "" &&
    !isAbsolute(below) &&
    below !== ".." &&
    !below.startsWith(".." + sep)
  );
}

/** Whether a write to `a` lands on `b`: the same file when both exist, else the same resolved path. */
async function samePath(a: string, b: string): Promise<boolean> {
  const [statA, statB] = await Promise.all([
    stat(a, { bigint: true }).catch(() => undefined),
    stat(b, { bigint: true }).catch(() => undefined),
  ]);
  if (
    statA !== undefined &&
    statB !== undefined &&
    statA.ino !== 0n &&
    statB.ino !== 0n
  ) {
    return statA.dev === statB.dev && statA.ino === statB.ino;
  }
  return (await landing(a)) === (await landing(b));
}

/**
 * The absolute path a write to `path` lands on: every symbolic link on the way
 * followed, a dangling one included, through the deepest ancestor that exists,
 * whatever below it does not.
 */
async function landing(path: string): Promise<string> {
  let head = resolve(path);
  const tail: string[] = [];
  let hops = 0;
  for (;;) {
    try {
      return join(await realpath(head), ...tail);
    } catch {
      const target = await readlink(head).catch(() => undefined);
      if (target !== undefined) {
        if (++hops >= MAX_SYMLINK_HOPS) {
          throw new RangeError(`${path}: too many levels of symbolic links`);
        }
        head = resolve(dirname(head), target);
        continue;
      }
      const parent = dirname(head);
      if (parent === head) {
        return join(head, ...tail);
      }
      tail.unshift(basename(head));
      head = parent;
    }
  }
}

/** Dangling symbolic links followed before a path is judged to loop, as the kernel's limit. */
const MAX_SYMLINK_HOPS = 40;

export async function load(
  path: string,
  options: ConnectOptions & ParseOptions = {},
): Promise<Model> {
  const connection = await connect(options);
  try {
    return await Model.parse(
      connection,
      { source: { case: "filePath", value: path } },
      options,
      true,
    );
  } catch (error) {
    await connection.close();
    throw error;
  }
}

/** Parses inline source over a connection of its own, which the model closes. */
export async function loads(
  source: string,
  options: ConnectOptions & ParseOptions = {},
): Promise<Model> {
  const connection = await connect(options);
  try {
    return await Model.parse(
      connection,
      { source: { case: "content", value: source } },
      options,
      true,
    );
  } catch (error) {
    await connection.close();
    throw error;
  }
}

// The release a connection requires, 'latest' resolved to the tag it names.
async function requiredRelease(
  options: ConnectOptions,
): Promise<string | undefined> {
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
    resolvePath: (path) => resolve(path),
  };
  // The open owns release from here on: a failure inside Connection.open
  // releases exactly once, so a refusal cannot drop another connection's hold.
  let transport: Transport;
  try {
    transport = transportFor(baseUrl(service.address), options);
  } catch (error) {
    await service.release();
    throw error;
  }
  return Connection.open({
    transport,
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
}

async function connectExternal(
  address: string,
  options: ConnectOptions,
): Promise<Connection> {
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
      resolvePath: (path) => resolve(path),
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
      throw new OpenSysMLError(
        "the gRPC protocol carries protobuf bodies; JSON needs the Connect protocol",
      );
    }
    return createGrpcTransport({
      baseUrl: url,
      interceptors: interceptors(options),
    });
  }
  return createConnectTransport({
    baseUrl: url,
    httpVersion: options.httpVersion ?? "2",
    useBinaryFormat: binary,
    interceptors: interceptors(options),
  });
}
