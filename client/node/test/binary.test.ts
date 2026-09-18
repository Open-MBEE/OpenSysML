// Finding the service binary, and what a release has to prove before it is installed.
// Every release here is served from a local HTTP server: no test touches the network.

import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { chmodSync, mkdtempSync, readFileSync, statSync, writeFileSync } from "node:fs";
import { mkdir, readFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { afterEach, beforeEach, test } from "node:test";
import {
  ALLOW_UNPINNED_ENV,
  BINARY_ENV,
  BinaryNotFoundError,
  ChecksumMismatchError,
  DownloadError,
  REPO_ENV,
  UnpinnedReleaseError,
  VERSION_ENV,
  binaryName,
  cachedBinaryPath,
  cachedRelease,
  downloadBinary,
  expectedDigest,
  metadataPath,
  pinnedDigest,
  platformPackage,
  releaseAssetName,
  releaseDownloadUrl,
  resolveBinary,
  resolveLatestVersion,
  staleCacheReason,
  unpinnedDownloadsAllowed,
  writeMetadata,
} from "../src/node/index.js";
import type { DownloadOptions } from "../src/node/index.js";
import { type Release, serveRelease } from "./release-server.js";

const REPO = "Test-Owner/OpenSysML";
const VERSION = "v0.9.9";
const BODY = Buffer.from("sysml-grpc test binary\n");
const DIGEST = createHash("sha256").update(BODY).digest("hex");

const saved = {
  binary: process.env[BINARY_ENV],
  version: process.env[VERSION_ENV],
  repo: process.env[REPO_ENV],
  allow: process.env[ALLOW_UNPINNED_ENV],
  path: process.env["PATH"],
};

// A binary or a release named in the environment would answer for the one under test.
beforeEach(() => {
  restore(BINARY_ENV, undefined);
  restore(VERSION_ENV, undefined);
  restore(REPO_ENV, undefined);
  restore(ALLOW_UNPINNED_ENV, undefined);
});

afterEach(() => {
  restore(BINARY_ENV, saved.binary);
  restore(VERSION_ENV, saved.version);
  restore(REPO_ENV, saved.repo);
  restore(ALLOW_UNPINNED_ENV, saved.allow);
  restore("PATH", saved.path);
});

test("the platform package and file name follow npm's os/cpu names", () => {
  assert.equal(platformPackage("linux", "x64"), "@opensysml/sysml-grpc-linux-x64");
  assert.equal(platformPackage("darwin", "arm64"), "@opensysml/sysml-grpc-darwin-arm64");
  assert.equal(platformPackage("win32", "x64"), "@opensysml/sysml-grpc-win32-x64");
  assert.equal(binaryName("linux"), "sysml-grpc");
  assert.equal(binaryName("win32"), "sysml-grpc.exe");
});

test("release assets are named for the platform, and unpublished pairs are refused", () => {
  assert.equal(releaseAssetName("linux", "x64"), "sysml-grpc-linux-amd64");
  assert.equal(releaseAssetName("linux", "arm64"), "sysml-grpc-linux-arm64");
  assert.equal(releaseAssetName("darwin", "x64"), "sysml-grpc-darwin-amd64");
  assert.equal(releaseAssetName("darwin", "arm64"), "sysml-grpc-darwin-arm64");
  assert.equal(releaseAssetName("win32", "x64"), "sysml-grpc-windows-amd64.exe");

  // No release carries these, and inventing a name would fetch a 404 instead of saying so.
  for (const [platform, arch] of [
    ["win32", "arm64"],
    ["linux", "riscv64"],
    ["aix", "x64"],
  ] as [string, string][]) {
    assert.throws(() => releaseAssetName(platform, arch), BinaryNotFoundError);
  }
});

test("$OPENSYSML_BINARY wins, and a path that is not executable is an error", async () => {
  const dir = mkdtempSync(join(tmpdir(), "binary-test-"));
  const executable = join(dir, "sysml-grpc");
  writeFileSync(executable, "#!/bin/sh\n");
  chmodSync(executable, 0o755);
  process.env[BINARY_ENV] = executable;
  assert.equal((await resolveBinary()).path, executable);
  assert.match((await resolveBinary()).source, /OPENSYSML_BINARY/);

  const plain = join(dir, "not-executable");
  writeFileSync(plain, "");
  chmodSync(plain, 0o644);
  process.env[BINARY_ENV] = plain;
  await assert.rejects(resolveBinary(), BinaryNotFoundError);
});

test("a binary on $PATH is used, and with nothing anywhere the error says so", async (t) => {
  if (process.platform === "win32") {
    t.skip("chmod and $PATH semantics differ on Windows");
    return;
  }
  const dir = mkdtempSync(join(tmpdir(), "binary-path-"));
  const empty = mkdtempSync(join(tmpdir(), "binary-cache-"));
  const executable = join(dir, binaryName());
  writeFileSync(executable, "#!/bin/sh\n");
  chmodSync(executable, 0o755);
  process.env["PATH"] = dir;
  assert.equal((await resolveBinary({ cacheDir: empty })).path, executable);

  process.env["PATH"] = empty;
  const error = await rejection(resolveBinary({ cacheDir: empty }));
  assert.ok(error instanceof BinaryNotFoundError);
  // The message must name every place looked, and how to ask for a download.
  assert.match(error.message, /OPENSYSML_BINARY/);
  assert.match(error.message, /@opensysml\/sysml-grpc-/);
  assert.match(error.message, new RegExp(empty.replaceAll(/[.*+?^${}()|[\]\\]/g, "\\$&")));
  assert.match(error.message, /OPENSYSML_GRPC_VERSION/);
  assert.doesNotMatch(error.message, /never downloads/);
});

test("$OPENSYSML_GITHUB_REPO chooses the repository, and the URLs follow it", () => {
  process.env[REPO_ENV] = REPO;
  assert.equal(
    releaseDownloadUrl(VERSION, "sysml-grpc-linux-amd64"),
    `https://github.com/${REPO}/releases/download/${VERSION}/sysml-grpc-linux-amd64`,
  );
});

test("a pinned release is downloaded, verified, cached and made executable", async (t) => {
  const release = await serveRelease(served());
  t.after(() => release.close());
  const options = pinned(release);

  const path = await downloadBinary(options);
  assert.equal(path, cachedBinaryPath(options));
  assert.deepEqual(await readFile(path), BODY);
  if (process.platform !== "win32") {
    assert.equal(statSync(path).mode & 0o777, 0o700);
  }

  // The metadata is the shape the Python client reads and writes.
  assert.deepEqual(JSON.parse(readFileSync(metadataPath(options), "utf8")), {
    version: VERSION,
    sha256: DIGEST,
    repo: REPO,
  });
  assert.equal(await cachedRelease(options), VERSION);
  assert.equal(release.requested.filter((url) => url.endsWith(".sha256")).length, 1);
  // A pinned release needs no manifest, so none is fetched.
  assert.deepEqual(
    release.requested.filter((url) => url.includes("SHA256SUMS")),
    [],
  );
});

test("a cached release is used again without downloading anything", async (t) => {
  const release = await serveRelease(served());
  t.after(() => release.close());
  const options = pinned(release);
  await downloadBinary(options);
  const requests = release.requested.length;

  const found = await resolveBinary({ ...options, version: VERSION });
  assert.equal(found.path, cachedBinaryPath(options));
  assert.match(found.source, /cached/);
  assert.equal(release.requested.length, requests);
});

test("a served checksum that disagrees with the pin is refused, and nothing is touched", async (t) => {
  const other = Buffer.from("another build entirely\n");
  const release = await serveRelease(served({ binary: other, checksum: sha256(other) }));
  t.after(() => release.close());
  const options = pinned(release);
  const cached = await cache(options, "cached binary\n", VERSION);

  const error = await rejection(downloadBinary(options));
  assert.ok(error instanceof ChecksumMismatchError);
  assert.match(error.message, /Checksum mismatch/);
  assert.match(error.message, new RegExp(DIGEST));

  // The cache is what it was, and the download left nothing behind.
  assert.deepEqual(await readFile(cached), Buffer.from("cached binary\n"));
  assert.equal(await cachedRelease(options), VERSION);
  await assert.rejects(readFile(`${cached}.tmp`));
  // The binary is never fetched once its checksum is known to be wrong.
  assert.deepEqual(
    release.requested.filter((url) => url.endsWith(releaseAssetName())),
    [],
  );
});

test("a body that does not match the checksum served with it is refused", async (t) => {
  // The sidecar agrees with the pin, so only the bytes are wrong: a truncated
  // download, or one substituted in flight.
  const release = await serveRelease(served({ binary: BODY.subarray(0, 8) }));
  t.after(() => release.close());
  const options = pinned(release);
  const cached = await cache(options, "cached binary\n", VERSION);

  const error = await rejection(downloadBinary(options));
  assert.ok(error instanceof ChecksumMismatchError);
  assert.deepEqual(await readFile(cached), Buffer.from("cached binary\n"));
  await assert.rejects(readFile(`${cached}.tmp`));
});

test("a release nothing pins a digest for is refused", async (t) => {
  const release = await serveRelease(served());
  t.after(() => release.close());
  const options: DownloadOptions = { ...unpinned(release), warn: () => undefined };

  const error = await rejection(downloadBinary(options));
  assert.ok(error instanceof UnpinnedReleaseError);
  assert.match(error.message, /pins no SHA-256 digest/);
  assert.match(error.message, new RegExp(VERSION));
  assert.match(error.message, new RegExp(ALLOW_UNPINNED_ENV));

  // Nothing pins this repository's release pipeline, so its manifest could not be
  // verified and is not fetched; only the sidecar is, and never the binary.
  assert.match(error.message, /knows no release pipeline identity/);
  assert.deepEqual(release.requested.map(asset), [`${releaseAssetName()}.sha256`]);
  await assert.rejects(readFile(cachedBinaryPath(options)));
});

for (const allowed of ["1", REPO]) {
  test(`$${ALLOW_UNPINNED_ENV}=${allowed} accepts the served checksum, with a warning`, async (t) => {
    const release = await serveRelease(served());
    t.after(() => release.close());
    process.env[ALLOW_UNPINNED_ENV] = allowed;
    const warnings: string[] = [];
    const options: DownloadOptions = {
      ...unpinned(release),
      warn: (message) => warnings.push(message),
    };

    const path = await downloadBinary(options);
    assert.deepEqual(await readFile(path), BODY);
    assert.equal(warnings.length, 1);
    // The warning has to say what this trust is worth.
    assert.match(warnings[0] ?? "", /detects corruption but not a compromised release/);
  });
}

test("the opt-in names one repository, and does not carry to another", () => {
  process.env[ALLOW_UNPINNED_ENV] = REPO;
  assert.equal(unpinnedDownloadsAllowed(REPO), true);
  assert.equal(unpinnedDownloadsAllowed("Someone-Else/OpenSysML"), false);
  process.env[ALLOW_UNPINNED_ENV] = "1";
  assert.equal(unpinnedDownloadsAllowed("Someone-Else/OpenSysML"), true);
  process.env[ALLOW_UNPINNED_ENV] = "0";
  assert.equal(unpinnedDownloadsAllowed(REPO), false);
  restore(ALLOW_UNPINNED_ENV, undefined);
  assert.equal(unpinnedDownloadsAllowed(REPO), false);
});

test("a pin beats the checksum served beside the binary, and a signature that contradicts it", () => {
  const table = { [REPO]: { [VERSION]: { "sysml-grpc-linux-amd64": DIGEST } } };
  const request = {
    version: VERSION,
    asset: "sysml-grpc-linux-amd64",
    githubRepo: REPO,
    pinnedDigests: table,
    warn: () => undefined,
  };
  assert.equal(pinnedDigest(VERSION, "sysml-grpc-linux-amd64", REPO, table), DIGEST);
  assert.equal(expectedDigest({ ...request, servedDigest: DIGEST }), DIGEST);
  assert.equal(
    expectedDigest({ ...request, servedDigest: DIGEST, verifiedDigest: DIGEST }),
    DIGEST,
  );
  assert.throws(
    () => expectedDigest({ ...request, servedDigest: "ab".repeat(32) }),
    ChecksumMismatchError,
  );
  assert.throws(
    () =>
      expectedDigest({
        ...request,
        servedDigest: DIGEST,
        verifiedDigest: "ab".repeat(32),
      }),
    (error: unknown) => {
      assert.ok(error instanceof ChecksumMismatchError);
      assert.ok(!(error instanceof UnpinnedReleaseError));
      assert.match(error.message, /signed SHA256SUMS.txt/);
      return true;
    },
  );
});

test("an unpinned release whose origin serves nonsense as a checksum is refused", () => {
  process.env[ALLOW_UNPINNED_ENV] = "1";
  assert.throws(
    () =>
      expectedDigest({
        version: VERSION,
        asset: "sysml-grpc-linux-amd64",
        servedDigest: "not-a-digest",
        githubRepo: REPO,
        pinnedDigests: {},
        warn: () => undefined,
      }),
    ChecksumMismatchError,
  );
});

test("latest resolves through the releases API, and is downloaded like any tag", async (t) => {
  const release = await serveRelease(served({ latest: VERSION }));
  t.after(() => release.close());
  const options = pinned(release);

  assert.equal(await resolveLatestVersion({ ...options, version: "latest" }), VERSION);
  const path = await downloadBinary({ ...options, version: "latest" });
  assert.deepEqual(await readFile(path), BODY);
  assert.equal(await cachedRelease(options), VERSION);
});

test("a cache of another release is replaced, with a warning saying which", async (t) => {
  const release = await serveRelease(served());
  t.after(() => release.close());
  const warnings: string[] = [];
  const options: DownloadOptions = {
    ...pinned(release),
    warn: (message) => warnings.push(message),
  };
  await cache(options, "an older build\n", "v0.0.1");

  assert.match((await staleCacheReason(VERSION, options)) ?? "", /is v0.0.1, but v0.9.9/);
  const found = await resolveBinary({ ...options, version: VERSION });
  assert.deepEqual(await readFile(found.path), BODY);
  assert.match(found.source, /downloaded/);
  assert.equal(warnings.length, 1);
  assert.match(warnings[0] ?? "", /Replacing the cached sysml-grpc/);
});

test("a cache the client did not fill is not read as the release asked for", async (t) => {
  const release = await serveRelease(served());
  t.after(() => release.close());
  const options = pinned(release);
  const path = cachedBinaryPath(options);
  await mkdir(join(path, ".."), { recursive: true });
  writeFileSync(path, "a hand-installed build\n");
  chmodSync(path, 0o755);

  assert.equal(await cachedRelease(options), undefined);
  assert.match((await staleCacheReason(VERSION, options)) ?? "", /was not downloaded by this client/);
  // Without a release asked for, whatever is cached stands.
  assert.match((await resolveBinary(unasked(options))).source, /cached/);
});

test("a cache whose bytes changed under it is not read as the release it records", async (t) => {
  const release = await serveRelease(served());
  t.after(() => release.close());
  const options = pinned(release);
  const path = await cache(options, "the release\n", VERSION);
  writeFileSync(path, "swapped out\n");

  assert.equal(await cachedRelease(options), undefined);
});

test("a working cache survives a release that cannot be downloaded", async (t) => {
  const release = await serveRelease({ ...served(), binary: undefined });
  t.after(() => release.close());
  const warnings: string[] = [];
  const options: DownloadOptions = {
    ...pinned(release),
    warn: (message) => warnings.push(message),
  };
  const cached = await cache(options, "an older build\n", "v0.0.1");

  const found = await resolveBinary({ ...options, version: VERSION });
  assert.equal(found.path, cached);
  assert.deepEqual(await readFile(cached), Buffer.from("an older build\n"));
  assert.match(warnings.join("\n"), /Keeping the cached sysml-grpc/);
});

test("a cache does not answer for a download that may have been tampered with", async (t) => {
  const other = Buffer.from("another build entirely\n");
  const release = await serveRelease(served({ binary: other, checksum: sha256(other) }));
  t.after(() => release.close());
  const options: DownloadOptions = { ...pinned(release), warn: () => undefined };
  await cache(options, "an older build\n", "v0.0.1");

  await assert.rejects(resolveBinary({ ...options, version: VERSION }), ChecksumMismatchError);
});

test("an unpinned release keeps a working cache instead of failing the client", async (t) => {
  const release = await serveRelease(served());
  t.after(() => release.close());
  const warnings: string[] = [];
  const options: DownloadOptions = {
    ...unpinned(release),
    warn: (message) => warnings.push(message),
  };
  const cached = await cache(options, "an older build\n", "v0.0.1");

  const found = await resolveBinary({ ...options, version: VERSION });
  assert.equal(found.path, cached);
  assert.match(warnings.join("\n"), /Keeping the cached sysml-grpc/);
});

test("a cache is kept when the releases API cannot say what latest is", async (t) => {
  const release = await serveRelease({ ...served(), latest: undefined });
  t.after(() => release.close());
  const options = pinned(release);
  await cache(options, "an older build\n", VERSION);

  await assert.rejects(resolveLatestVersion({ ...options }), DownloadError);
  // An unreachable API is no reason to discard a binary that works.
  assert.equal(await staleCacheReason("latest", options), undefined);
  assert.match((await resolveBinary({ ...options, version: "latest" })).source, /cached/);
});

test("$OPENSYSML_GRPC_VERSION asks for the release the caller did not name", async (t) => {
  const release = await serveRelease(served());
  t.after(() => release.close());
  const options = pinned(release);
  process.env[VERSION_ENV] = VERSION;

  const found = await resolveBinary(unasked(options));
  assert.deepEqual(await readFile(found.path), BODY);
});

/** A release serving the fixed test body, with everything a pinned download needs. */
function served(overrides: Partial<Release> = {}): Release {
  return {
    version: VERSION,
    repo: REPO,
    asset: releaseAssetName(),
    binary: BODY,
    checksum: DIGEST,
    ...overrides,
  };
}

/** Options for a download of a release this test run pins a digest for. */
function pinned(release: { url: string; apiUrl: string }): DownloadOptions {
  return {
    ...unpinned(release),
    pinnedDigests: { [REPO]: { [VERSION]: { [releaseAssetName()]: DIGEST } } },
  };
}

/** Options for a download of a release nothing pins, and nothing signs. */
function unpinned(release: { url: string; apiUrl: string }): DownloadOptions {
  return {
    version: VERSION,
    githubRepo: REPO,
    releasesBaseUrl: release.url,
    apiBaseUrl: release.apiUrl,
    cacheDir: mkdtempSync(join(tmpdir(), "binary-cache-")),
    pinnedDigests: {},
  };
}

/** A cached binary of a release, as an earlier download would have left it. */
async function cache(options: DownloadOptions, content: string, version: string): Promise<string> {
  const path = cachedBinaryPath(options);
  await mkdir(join(path, ".."), { recursive: true });
  writeFileSync(path, content);
  chmodSync(path, 0o700);
  await writeMetadata(version, sha256(Buffer.from(content)), options);
  return path;
}

function sha256(content: Buffer): string {
  return createHash("sha256").update(content).digest("hex");
}

/** The asset a request was for, so a test can assert on what was fetched. */
function asset(url: string): string {
  return url.slice(url.lastIndexOf("/") + 1);
}

/** The same options, with no release asked for. */
function unasked(options: DownloadOptions): DownloadOptions {
  const request = { ...options };
  delete request.version;
  return request;
}

async function rejection(promise: Promise<unknown>): Promise<Error> {
  try {
    await promise;
  } catch (error) {
    assert.ok(error instanceof Error);
    return error;
  }
  assert.fail("expected a rejection");
}

function restore(name: string, value: string | undefined): void {
  if (value === undefined) {
    Reflect.deleteProperty(process.env, name);
  } else {
    process.env[name] = value;
  }
}
