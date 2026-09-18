// Finding the sysml-grpc binary, and fetching one when a release is asked for:
// $OPENSYSML_BINARY, then the optional per-platform npm package, then a release
// download shared with the Python client's cache, then $PATH. A download is only
// ever installed once its bytes match a digest this client shipped, or one from a
// checksum manifest the release pipeline signed.

import { createHash } from "node:crypto";
import { accessSync, constants, existsSync, readFileSync } from "node:fs";
import { chmod, mkdir, readFile, rename, unlink, writeFile } from "node:fs/promises";
import { createRequire } from "node:module";
import { homedir } from "node:os";
import { delimiter, dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import {
  ChecksumMismatchError,
  DownloadError,
  OpenSysMLError,
  UnpinnedReleaseError,
  UnsignedReleaseError,
} from "../core/errors.js";
import {
  BUNDLE_ASSET,
  MANIFEST_ASSET,
  ReleaseSigner,
  signerFor,
  verifiedManifestDigest,
} from "./signing.js";

/** Names a build of sysml-grpc to use, in place of any package, download or cache. */
export const BINARY_ENV = "OPENSYSML_BINARY";

/** Names the release to download, when the caller asks for none. */
export const VERSION_ENV = "OPENSYSML_GRPC_VERSION";

/** Overrides the repository releases are downloaded from. */
export const REPO_ENV = "OPENSYSML_GITHUB_REPO";

/** Repository whose unpinned downloads may be accepted (`1` for any): same-origin trust. */
export const ALLOW_UNPINNED_ENV = "OPENSYSML_ALLOW_UNPINNED_DOWNLOAD";

/** Repository releases are downloaded from unless $OPENSYSML_GITHUB_REPO names another. */
export const DEFAULT_GITHUB_REPO = "Open-MBEE/OpenSysML";

/** Origin a release publishes its assets at, and the API the latest tag is read from. */
export const RELEASES_BASE_URL = "https://github.com";
export const API_BASE_URL = "https://api.github.com";

/** Per-request timeout: this runs while a service is starting, so it must not hang. */
export const NETWORK_TIMEOUT_MS = 15_000;

const SHA256 = /^[0-9a-f]{64}$/;

/** The pinned digest table, keyed by repository, release tag and asset name. */
export type PinnedDigests = Readonly<
  Record<string, Readonly<Record<string, Readonly<Record<string, string>>>>>
>;

/** Where a binary was found, and how. */
export interface Binary {
  path: string;
  /** Human-readable provenance, used in error messages and ServerInfo.origin. */
  source: string;
}

/** What the cached binary was downloaded from, shared with the Python client. */
export interface CacheMetadata {
  version: string;
  sha256: string;
  repo: string;
}

/** How a release is downloaded. Every field has a default; tests supply the seams. */
export interface DownloadOptions {
  /** Release tag to install, or 'latest'; $OPENSYSML_GRPC_VERSION when omitted. */
  version?: string;
  /** Repository to download from; $OPENSYSML_GITHUB_REPO or the default when omitted. */
  githubRepo?: string;
  /** Origin release assets are published at. */
  releasesBaseUrl?: string;
  /** GitHub API the latest release tag is read from. */
  apiBaseUrl?: string;
  /** Directory the binary and its metadata are cached in. */
  cacheDir?: string;
  /** Platform the release asset is chosen for; this process's by default. */
  platform?: string;
  /** Architecture the release asset is chosen for; this process's by default. */
  arch?: string;
  /** Identity the release manifest's signature must carry; the pinned one when omitted. */
  signer?: ReleaseSigner;
  /** Digest table to verify against; the one this package ships when omitted. */
  pinnedDigests?: PinnedDigests;
  /** Where warnings go; Node's process warnings when omitted. */
  warn?: (message: string) => void;
}

/** No usable binary was found, and none could be fetched. */
export class BinaryNotFoundError extends OpenSysMLError {}

/**
 * The digests this package ships, synced from clients/release-digests.json: a pin
 * resolved from outside the published artifact would not be a pin.
 */
export const PINNED_SHA256: PinnedDigests = readPinnedDigests(
  join(packageRoot(), "release-digests.json"),
);

/** The npm package that carries the service binary for this platform. */
export function platformPackage(
  platform: string = process.platform,
  arch: string = process.arch,
): string {
  return `@opensysml/sysml-grpc-${platform}-${arch}`;
}

/** The binary's file name on this platform. */
export function binaryName(platform: string = process.platform): string {
  return platform === "win32" ? "sysml-grpc.exe" : "sysml-grpc";
}

/** Repository releases are downloaded from. */
export function defaultGithubRepo(): string {
  const named = process.env[REPO_ENV];
  return named !== undefined && named !== "" ? named : DEFAULT_GITHUB_REPO;
}

/** Name a release publishes the service binary for a platform under. */
export function releaseAssetName(
  platform: string = process.platform,
  arch: string = process.arch,
): string {
  const goos = own(GOOS, platform);
  const goarch = own(GOARCH, arch);
  if (goos === undefined || goarch === undefined || !SUPPORTED_PAIRS.includes(`${goos}-${goarch}`)) {
    throw new BinaryNotFoundError(
      `no sysml-grpc release is published for ${platform}/${arch}; releases carry ` +
        `${SUPPORTED_PAIRS.join(", ")}. Build the service yourself (make build-grpc) ` +
        `and set $${BINARY_ENV} to it.`,
    );
  }
  const name = `sysml-grpc-${goos}-${goarch}`;
  return goos === "windows" ? `${name}.exe` : name;
}

/** The digest this release of the client pins for a release asset. */
export function pinnedDigest(
  version: string,
  asset: string,
  githubRepo?: string,
  table: PinnedDigests = PINNED_SHA256,
): string | undefined {
  const repo = githubRepo ?? defaultGithubRepo();
  return own(own(own(table, repo), version), asset);
}

/** Whether an unpinned download from a repository may fall back to same-origin trust. */
export function unpinnedDownloadsAllowed(githubRepo?: string): boolean {
  const allowed = (process.env[ALLOW_UNPINNED_ENV] ?? "").trim();
  const lowered = allowed.toLowerCase();
  if (["", "0", "false", "no"].includes(lowered)) {
    return false;
  }
  if (["1", "true", "yes"].includes(lowered)) {
    return true;
  }
  // Naming repositories keeps the trust with the fork it is granted for.
  const repo = githubRepo ?? defaultGithubRepo();
  return allowed.split(",").some((named) => named.trim() === repo);
}

/**
 * The digest a download must have: the pin wherever there is one, else the signed
 * manifest's, else nothing, unless same-origin trust was allowed explicitly.
 */
export function expectedDigest(request: {
  version: string;
  asset: string;
  servedDigest: string;
  githubRepo?: string;
  /** Digest from the release's signed checksum manifest, once its signature verified. */
  verifiedDigest?: string;
  /** Why there is no verified digest, said in the refusal when there is no pin either. */
  unverifiedReason?: string;
  pinnedDigests?: PinnedDigests;
  warn?: (message: string) => void;
}): string {
  const repo = request.githubRepo ?? defaultGithubRepo();
  const { version, asset, servedDigest, verifiedDigest } = request;
  const pinned = pinnedDigest(version, asset, repo, request.pinnedDigests ?? PINNED_SHA256);

  if (pinned === undefined) {
    if (verifiedDigest !== undefined) {
      // Signed by the pipeline that built the release, so it is not the origin
      // vouching for itself: as good as a pin.
      return verifiedDigest;
    }
    if (!unpinnedDownloadsAllowed(repo)) {
      const unverified =
        request.unverifiedReason === undefined ? "" : ` and ${request.unverifiedReason}`;
      throw new UnpinnedReleaseError(
        `this client pins no SHA-256 digest for ${asset} of ${version} of ${repo}` +
          `${unverified}, so the only checksum available is the one served beside the ` +
          `binary, which a compromised release would serve too. Upgrade the client to a ` +
          `release that pins ${version}, ask for a pinned release with version, or ` +
          `accept same-origin trust for this repository by setting ` +
          `$${ALLOW_UNPINNED_ENV}=${repo} (or =1 for any repository).`,
      );
    }
    if (!SHA256.test(servedDigest)) {
      throw new ChecksumMismatchError(
        `the checksum ${repo} serves beside ${asset} of ${version} is not a SHA-256 ` +
          `digest (${JSON.stringify(servedDigest)}); nothing was installed.`,
      );
    }
    warnWith(request.warn)(
      `this client pins no digest for ${asset} of ${version} of ${repo}; verifying it ` +
        `against the checksum served beside it, which detects corruption but not a ` +
        `compromised release ($${ALLOW_UNPINNED_ENV} is set).`,
    );
    return servedDigest;
  }

  if (verifiedDigest !== undefined && verifiedDigest !== pinned) {
    throw new ChecksumMismatchError(
      `Checksum mismatch for ${asset} of ${version}: the signed ${MANIFEST_ASSET} of ` +
        `${repo} lists ${verifiedDigest}, but this client pins ${pinned}. The release ` +
        `was rebuilt after this client pinned it; it was not installed.`,
    );
  }
  if (servedDigest !== pinned) {
    throw new ChecksumMismatchError(
      `Checksum mismatch for ${asset} of ${version}: ${repo} serves ${servedDigest}, ` +
        `but this client pins ${pinned}. The release was republished with another ` +
        `binary, or the download is being tampered with; it was not installed.`,
    );
  }
  return pinned;
}

/** URL a release publishes an asset at. */
export function releaseDownloadUrl(
  version: string,
  asset: string,
  options: DownloadOptions = {},
): string {
  const repo = options.githubRepo ?? defaultGithubRepo();
  const base = options.releasesBaseUrl ?? RELEASES_BASE_URL;
  return `${base}/${repo}/releases/download/${version}/${asset}`;
}

/** The digest for an asset from the release's signed checksum manifest. */
export async function signedManifestDigest(
  version: string,
  asset: string,
  options: DownloadOptions = {},
): Promise<string> {
  const repo = options.githubRepo ?? defaultGithubRepo();
  const signer = options.signer ?? signerFor(repo);
  if (signer === undefined) {
    throw new UnsignedReleaseError(
      `this client knows no release pipeline identity for ${repo}, so a signed ` +
        `${MANIFEST_ASSET} of it would not be verifiable`,
    );
  }

  const published = async (name: string): Promise<Buffer> => {
    const url = releaseDownloadUrl(version, name, { ...options, githubRepo: repo });
    try {
      return await fetchBytes(url);
    } catch (cause) {
      throw new UnsignedReleaseError(
        `${version} of ${repo} publishes no readable ${name} (${url}: ` +
          `${describe(cause)}), so its checksums carry no signature to verify`,
        { cause },
      );
    }
  };

  const manifest = await published(MANIFEST_ASSET);
  const bundle = await published(BUNDLE_ASSET);
  return verifiedManifestDigest(manifest, bundle, asset, signer);
}

/** Resolve the tag of the repository's latest published release. */
export async function resolveLatestVersion(options: DownloadOptions = {}): Promise<string> {
  const repo = options.githubRepo ?? defaultGithubRepo();
  const url = `${options.apiBaseUrl ?? API_BASE_URL}/repos/${repo}/releases/latest`;
  let release: unknown;
  try {
    release = JSON.parse((await fetchBytes(url)).toString("utf8"));
  } catch (cause) {
    throw new DownloadError(`Failed to resolve latest release from ${url}: ${describe(cause)}`, {
      cause,
    });
  }
  const tag =
    typeof release === "object" && release !== null
      ? (release as { tag_name?: unknown }).tag_name
      : undefined;
  if (typeof tag !== "string" || tag === "") {
    throw new DownloadError(`Latest release of ${repo} has no tag name`);
  }
  return tag;
}

/** Path the downloaded binary is cached at, shared with the Python client. */
export function cachedBinaryPath(options: DownloadOptions = {}): string {
  return join(options.cacheDir ?? join(homedir(), ".opensysml", "bin"), binaryName());
}

/** Path of the record of which release the cached binary was downloaded from. */
export function metadataPath(options: DownloadOptions = {}): string {
  return `${cachedBinaryPath(options)}.json`;
}

/** Read the record beside the cached binary, or nothing when it says nothing. */
export async function readMetadata(options: DownloadOptions = {}): Promise<CacheMetadata | undefined> {
  let parsed: unknown;
  try {
    parsed = JSON.parse(await readFile(metadataPath(options), "utf8"));
  } catch {
    return undefined;
  }
  if (typeof parsed !== "object" || parsed === null) {
    return undefined;
  }
  const record = parsed as Record<string, unknown>;
  const { version, sha256, repo } = record;
  if (typeof version !== "string" || typeof sha256 !== "string" || typeof repo !== "string") {
    return undefined;
  }
  return { version, sha256, repo };
}

/** Record which release of which repository the cached binary is, and its digest. */
export async function writeMetadata(
  version: string,
  sha256: string,
  options: DownloadOptions = {},
): Promise<void> {
  const path = metadataPath(options);
  await mkdir(dirname(path), { recursive: true });
  const metadata: CacheMetadata = {
    version,
    sha256,
    repo: options.githubRepo ?? defaultGithubRepo(),
  };
  await writeFile(path, JSON.stringify(metadata));
}

/**
 * Release tag the cached binary was downloaded from, if from this repository: the
 * digest is re-checked, so a binary swapped in by hand does not answer for it.
 */
export async function cachedRelease(options: DownloadOptions = {}): Promise<string | undefined> {
  const recorded = await readMetadata(options);
  if (recorded === undefined) {
    return undefined;
  }
  if (recorded.repo !== (options.githubRepo ?? defaultGithubRepo())) {
    return undefined;
  }
  return (await verifyChecksum(cachedBinaryPath(options), recorded.sha256))
    ? recorded.version
    : undefined;
}

/**
 * Why the cached binary is not the release asked for, or nothing when it is: an
 * older cache would otherwise fail on whatever that build cannot do.
 */
export async function staleCacheReason(
  version: string | undefined,
  options: DownloadOptions = {},
): Promise<string | undefined> {
  if (version === undefined) {
    return undefined;
  }
  const repo = options.githubRepo ?? defaultGithubRepo();
  let wanted = version;
  if (wanted === "latest") {
    try {
      wanted = await resolveLatestVersion({ ...options, githubRepo: repo });
    } catch (error) {
      if (error instanceof DownloadError) {
        // Unreachable releases are no reason to discard a working cache.
        return undefined;
      }
      throw error;
    }
  }

  const have = await cachedRelease({ ...options, githubRepo: repo });
  if (have === wanted) {
    return undefined;
  }
  const path = cachedBinaryPath(options);
  if (have === undefined) {
    const recorded = await readMetadata(options);
    if (recorded !== undefined && recorded.repo !== repo) {
      return (
        `the binary cached at ${path} was downloaded from ${recorded.repo}, but ` +
        `${wanted} of ${repo} was asked for`
      );
    }
    return (
      `the binary cached at ${path} was not downloaded by this client, so which ` +
      `release it is cannot be told, and ${wanted} was asked for`
    );
  }
  return `the binary cached at ${path} is ${have}, but ${wanted} was asked for`;
}

/**
 * Download a release's service binary and install it in the cache, replacing what is
 * there only once the bytes match the digest expected of them.
 */
export async function downloadBinary(options: DownloadOptions = {}): Promise<string> {
  const repo = options.githubRepo ?? defaultGithubRepo();
  const asked = options.version ?? "latest";
  const version = asked === "latest" ? await resolveLatestVersion({ ...options, githubRepo: repo }) : asked;
  const settled: DownloadOptions = { ...options, githubRepo: repo, version };

  const asset = releaseAssetName(options.platform ?? process.platform, options.arch ?? process.arch);
  const binaryUrl = releaseDownloadUrl(version, asset, settled);
  const checksumUrl = releaseDownloadUrl(version, `${asset}.sha256`, settled);

  const binaryPath = cachedBinaryPath(settled);
  const temporaryPath = `${binaryPath}.tmp`;
  await mkdir(dirname(binaryPath), { recursive: true });

  let served: string;
  try {
    // The sidecar's format is "hexdigest  filename".
    served = (await fetchBytes(checksumUrl)).toString("utf8").trim().split(/\s+/)[0] ?? "";
  } catch (cause) {
    throw new DownloadError(`Failed to download ${checksumUrl}: ${describe(cause)}`, { cause });
  }

  // A release nothing is pinned for is vouched for by the signature on its checksum
  // manifest instead, which the origin cannot forge.
  let verifiedDigest: string | undefined;
  let unverifiedReason: string | undefined;
  if (pinnedDigest(version, asset, repo, settled.pinnedDigests ?? PINNED_SHA256) === undefined) {
    try {
      verifiedDigest = await signedManifestDigest(version, asset, settled);
    } catch (error) {
      if (!(error instanceof UnsignedReleaseError)) {
        throw error;
      }
      unverifiedReason = error.message;
    }
  }

  const expected = expectedDigest({
    version,
    asset,
    servedDigest: served.toLowerCase(),
    githubRepo: repo,
    ...(verifiedDigest === undefined ? {} : { verifiedDigest }),
    ...(unverifiedReason === undefined ? {} : { unverifiedReason }),
    ...(settled.pinnedDigests === undefined ? {} : { pinnedDigests: settled.pinnedDigests }),
    ...(settled.warn === undefined ? {} : { warn: settled.warn }),
  });

  let downloaded: Buffer;
  try {
    downloaded = await fetchBytes(binaryUrl);
  } catch (cause) {
    throw new DownloadError(`Failed to download binary from ${binaryUrl}: ${describe(cause)}`, {
      cause,
    });
  }
  await writeFile(temporaryPath, downloaded);

  if (!(await verifyChecksum(temporaryPath, expected))) {
    await remove(temporaryPath);
    throw new ChecksumMismatchError(
      `Checksum mismatch for ${asset}. Expected ${expected}, but the download does ` +
        `not match. It may be corrupted or tampered with; it was not installed.`,
    );
  }

  if (process.platform !== "win32") {
    // Before it is in place, so the cache is never a file no one may run.
    // The cache is this user's, so no one else needs it.
    await chmod(temporaryPath, 0o700);
  }
  try {
    // rename overwrites the cache being replaced, and is atomic within a directory.
    await rename(temporaryPath, binaryPath);
  } catch (cause) {
    await remove(temporaryPath);
    throw new DownloadError(
      `Downloaded ${version} but could not install it at ${binaryPath}: ` +
        `${describe(cause)}. A running service holding that file is the usual cause.`,
      { cause },
    );
  }
  await writeMetadata(version, expected, settled);
  return binaryPath;
}

/** Whether a file's SHA-256 digest is the one expected of it. */
export async function verifyChecksum(path: string, expectedSha256: string): Promise<boolean> {
  let content: Buffer;
  try {
    content = await readFile(path);
  } catch {
    return false;
  }
  return createHash("sha256").update(content).digest("hex") === expectedSha256;
}

/**
 * Finds a sysml-grpc binary: $OPENSYSML_BINARY, then the platform package, then a
 * release download into the cache the Python client shares, then $PATH. A release is
 * downloaded only when one is asked for, by `version` or $OPENSYSML_GRPC_VERSION.
 */
export async function resolveBinary(options: DownloadOptions = {}): Promise<Binary> {
  const looked: string[] = [];

  const named = process.env[BINARY_ENV];
  if (named !== undefined && named !== "") {
    if (!isExecutable(named)) {
      throw new BinaryNotFoundError(
        `$${BINARY_ENV} names ${named}, which is not an executable file`,
      );
    }
    return { path: named, source: `${named} (from $${BINARY_ENV})` };
  }
  looked.push(`$${BINARY_ENV}`);

  const packaged = fromPlatformPackage();
  if (packaged !== undefined) {
    return packaged;
  }
  looked.push(`the ${platformPackage()} package`);

  const installed = await fromRelease(options);
  if (installed !== undefined) {
    return installed;
  }
  looked.push(cachedBinaryPath(options));

  const onPath = fromPath();
  if (onPath !== undefined) {
    return onPath;
  }
  looked.push("$PATH");

  throw new BinaryNotFoundError(
    `no sysml-grpc binary: looked at ${looked.join(", ")}.\n` +
      `  fix: install the service for this platform (npm install ${platformPackage()}), or\n` +
      `       ask for a release to download by setting $${VERSION_ENV} (e.g. latest), or\n` +
      `       build it (make build-grpc) and set $${BINARY_ENV} to the result, or\n` +
      `       start a service yourself and pass its address to connect().`,
  );
}

/** The cached binary, or a release downloaded into the cache when one is asked for. */
async function fromRelease(options: DownloadOptions): Promise<Binary | undefined> {
  const asked = options.version ?? process.env[VERSION_ENV];
  const version = asked === undefined || asked === "" ? undefined : asked;
  const request: DownloadOptions = { ...options, ...(version === undefined ? {} : { version }) };
  const path = cachedBinaryPath(request);
  const warn = warnWith(options.warn);

  let cached: Binary | undefined;
  if (isExecutable(path)) {
    const stale = await staleCacheReason(version, request);
    if (stale === undefined) {
      return { path, source: `${path} (cached)` };
    }
    cached = { path, source: `${path} (cached, kept)` };
    warn(`Replacing the cached sysml-grpc: ${stale}. Downloading ${version ?? "latest"}.`);
  }

  if (version === undefined) {
    // Nothing cached and no release asked for: auto-download is off.
    return undefined;
  }

  try {
    const installed = await downloadBinary(request);
    return { path: installed, source: `${installed} (downloaded ${version} from ${request.githubRepo ?? defaultGithubRepo()})` };
  } catch (error) {
    if (error instanceof UnpinnedReleaseError) {
      // A release this client pins nothing for contradicts nothing, so a working
      // cache stands.
      if (cached === undefined) {
        throw error;
      }
      warn(
        `Keeping the cached sysml-grpc at ${cached.path}: ${version} was not downloaded ` +
          `(${error.message}). It may be an older release than asked for.`,
      );
      return cached;
    }
    if (error instanceof ChecksumMismatchError) {
      // A download that may have been tampered with is never answered from the cache.
      throw error;
    }
    if (error instanceof DownloadError && cached !== undefined) {
      // A release with no binary to fetch is no reason to lose a working one.
      warn(
        `Keeping the cached sysml-grpc at ${cached.path}: ${version} could not be ` +
          `downloaded (${error.message}). It may be an older release than asked for.`,
      );
      return cached;
    }
    throw error;
  }
}

function fromPlatformPackage(): Binary | undefined {
  const name = platformPackage();
  const require_ = createRequire(import.meta.url);
  let manifest: string;
  try {
    manifest = require_.resolve(`${name}/package.json`);
  } catch {
    // An optional dependency npm skipped for this platform, or none published for it.
    return undefined;
  }
  const path = join(manifest, "..", "bin", binaryName());
  if (!isExecutable(path)) {
    throw new BinaryNotFoundError(
      `the ${name} package is installed but holds no executable at ${path}; reinstall it`,
    );
  }
  return { path, source: `${path} (from ${name})` };
}

function fromPath(): Binary | undefined {
  const directories = (process.env["PATH"] ?? "").split(delimiter).filter((entry) => entry !== "");
  for (const directory of directories) {
    const candidate = join(directory, binaryName());
    if (isExecutable(candidate)) {
      return { path: candidate, source: `${candidate} (on $PATH)` };
    }
  }
  return undefined;
}

function isExecutable(path: string): boolean {
  try {
    accessSync(path, constants.X_OK);
    return true;
  } catch {
    return false;
  }
}

/** Release assets are published for these GOOS/GOARCH pairs, and no others. */
const SUPPORTED_PAIRS = ["linux-amd64", "linux-arm64", "darwin-amd64", "darwin-arm64", "windows-amd64"];

const GOOS: Readonly<Record<string, string>> = {
  linux: "linux",
  darwin: "darwin",
  win32: "windows",
};

const GOARCH: Readonly<Record<string, string>> = {
  x64: "amd64",
  arm64: "arm64",
};

async function fetchBytes(url: string): Promise<Buffer> {
  const response = await fetch(url, {
    redirect: "follow",
    signal: AbortSignal.timeout(NETWORK_TIMEOUT_MS),
  });
  if (!response.ok) {
    throw new Error(`HTTP ${response.status} ${response.statusText}`);
  }
  return Buffer.from(await response.arrayBuffer());
}

async function remove(path: string): Promise<void> {
  try {
    await unlink(path);
  } catch {
    // Already gone, which is all this was for.
  }
}

function warnWith(warn: ((message: string) => void) | undefined): (message: string) => void {
  return (
    warn ??
    ((message: string) => {
      process.emitWarning(message);
    })
  );
}

function describe(cause: unknown): string {
  return cause instanceof Error ? `${cause.name}: ${cause.message}` : String(cause);
}

/** The directory this package's own files sit in, which is what npm publishes. */
function packageRoot(): string {
  let directory = dirname(fileURLToPath(import.meta.url));
  for (;;) {
    if (existsSync(join(directory, "package.json"))) {
      return directory;
    }
    const parent = dirname(directory);
    if (parent === directory) {
      throw new OpenSysMLError(
        "this client's package directory could not be found, so the pinned release " +
          "digests it ships cannot be read",
      );
    }
    directory = parent;
  }
}

function readPinnedDigests(path: string): PinnedDigests {
  const parsed: unknown = JSON.parse(readFileSync(path, "utf8"));
  const table: Record<string, Record<string, Record<string, string>>> = {};
  for (const [repo, releases] of Object.entries(asObject(parsed, path))) {
    const byVersion: Record<string, Record<string, string>> = {};
    for (const [version, assets] of Object.entries(asObject(releases, path))) {
      const byAsset: Record<string, string> = {};
      for (const [asset, digest] of Object.entries(asObject(assets, path))) {
        if (typeof digest !== "string" || !SHA256.test(digest)) {
          throw new OpenSysMLError(
            `${path} pins ${JSON.stringify(digest)} for ${asset} of ${version} of ` +
              `${repo}, which is not a SHA-256 digest`,
          );
        }
        byAsset[asset] = digest;
      }
      byVersion[version] = byAsset;
    }
    table[repo] = byVersion;
  }
  return table;
}

function asObject(value: unknown, path: string): Record<string, unknown> {
  if (typeof value !== "object" || value === null || Array.isArray(value)) {
    throw new OpenSysMLError(`${path} is not the pinned digest table this client expects`);
  }
  return value as Record<string, unknown>;
}

function own<T>(record: Readonly<Record<string, T>> | undefined, key: string): T | undefined {
  if (record === undefined || !Object.hasOwn(record, key)) {
    return undefined;
  }
  return record[key];
}
