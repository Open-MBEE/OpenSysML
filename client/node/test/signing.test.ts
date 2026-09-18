// Verification of a release's signed checksum manifest.
//
// Every test here is offline: the bundles under test/fixtures/signed_release were
// recorded by clients/python/scripts/make_signed_release_fixture.py against a root
// of trust the fixtures carry, so nothing reaches Sigstore's production instance.

import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { mkdtempSync, readFileSync } from "node:fs";
import { readFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { afterEach, beforeEach, test } from "node:test";
import {
  ALLOW_UNPINNED_ENV,
  BUNDLE_ASSET,
  ChecksumMismatchError,
  MANIFEST_ASSET,
  ReleaseSigner,
  SIGNED_MANIFEST_SIGNERS,
  UnpinnedReleaseError,
  UnsignedReleaseError,
  cachedBinaryPath,
  cachedRelease,
  downloadBinary,
  manifestDigest,
  metadataPath,
  signedManifestDigest,
  signerFor,
  verifiedManifestDigest,
  verifyManifest,
} from "../src/node/index.js";
import type { DownloadOptions } from "../src/node/index.js";
import { type Release, serveRelease } from "./release-server.js";

// These tests run compiled, from build/test, so the fixtures come from the source tree.
const FIXTURES = join(import.meta.dirname, "..", "..", "test", "fixtures", "signed_release");
const REPO = "Test-Owner/OpenSysML";
const VERSION = "v9.9.9";
/** The asset the recorded manifest covers, and the bytes it covers it for. */
const ASSET = "sysml-grpc-linux-amd64";
const BODY = fixture(ASSET);
const DIGEST = createHash("sha256").update(BODY).digest("hex");

const identity = JSON.parse(readFileSync(join(FIXTURES, "identity.json"), "utf8")) as {
  issuer: string;
  project: string;
  definition: string;
  other_issuer: string;
  other_project: string;
};

/** The identity the fixtures were signed with, against their own root of trust. */
const SIGNER = new ReleaseSigner({
  issuer: identity.issuer,
  project: identity.project,
  trustedRootPath: join(FIXTURES, "trusted_root.json"),
});

const saved = { allow: process.env[ALLOW_UNPINNED_ENV] };

// Same-origin trust granted in the environment would hide what a signature is for.
beforeEach(() => {
  Reflect.deleteProperty(process.env, ALLOW_UNPINNED_ENV);
});

afterEach(() => {
  if (saved.allow === undefined) {
    Reflect.deleteProperty(process.env, ALLOW_UNPINNED_ENV);
  } else {
    process.env[ALLOW_UNPINNED_ENV] = saved.allow;
  }
});

test("the signer for the release repository is pinned, and no other repository has one", () => {
  const pinned = signerFor("Open-MBEE/OpenSysML");
  assert.ok(pinned !== undefined);
  assert.equal(pinned.issuer, identity.issuer);
  assert.equal(pinned.project, identity.project);
  assert.deepEqual(Object.keys(SIGNED_MANIFEST_SIGNERS), ["Open-MBEE/OpenSysML"]);
  assert.equal(signerFor("Someone-Else/OpenSysML"), undefined);
  // Nothing is verified against Sigstore's production instance in tests, but the
  // pinned signer is what a real download uses.
  assert.equal(pinned.trustedRootPath, undefined);
});

test("a manifest signed by the release pipeline verifies, and its digest is used", async () => {
  await verifyManifest(fixture(MANIFEST_ASSET), fixture(BUNDLE_ASSET), SIGNER);
  assert.equal(
    await verifiedManifestDigest(fixture(MANIFEST_ASSET), fixture(BUNDLE_ASSET), ASSET, SIGNER),
    DIGEST,
  );
});

test("a subject naming the project's pipeline is accepted, and another project's is not", async () => {
  const definition = new ReleaseSigner({
    issuer: identity.issuer,
    project: identity.project,
    definition: identity.definition,
    trustedRootPath: join(FIXTURES, "trusted_root.json"),
  });
  await verifyManifest(fixture(MANIFEST_ASSET), fixture(BUNDLE_ASSET), definition);

  const wrong = new ReleaseSigner({
    issuer: identity.issuer,
    project: identity.project,
    definition: "00000000-0000-4000-8000-000000000000",
    trustedRootPath: join(FIXTURES, "trusted_root.json"),
  });
  await assert.rejects(
    verifyManifest(fixture(MANIFEST_ASSET), fixture(BUNDLE_ASSET), wrong),
    signatureFailure,
  );
});

test("a manifest signed by another organization's pipeline is refused", async () => {
  await assert.rejects(
    verifyManifest(
      fixture(MANIFEST_ASSET),
      fixture("SHA256SUMS.txt.other-identity.bundle"),
      SIGNER,
    ),
    signatureFailure,
  );
  // The identity is the whole point: the same bundle verifies for its own signer.
  const other = new ReleaseSigner({
    issuer: identity.other_issuer,
    project: identity.other_project,
    trustedRootPath: join(FIXTURES, "trusted_root.json"),
  });
  await verifyManifest(fixture(MANIFEST_ASSET), fixture("SHA256SUMS.txt.other-identity.bundle"), other);
});

test("a manifest changed after it was signed is refused", async () => {
  const tampered = Buffer.from(
    fixture(MANIFEST_ASSET).toString("utf8").replace(DIGEST, "ab".repeat(32)),
  );
  await assert.rejects(verifyManifest(tampered, fixture(BUNDLE_ASSET), SIGNER), signatureFailure);
});

test("a certificate that had expired when the log integrated the entry is refused", async () => {
  await assert.rejects(
    verifyManifest(fixture(MANIFEST_ASSET), fixture("SHA256SUMS.txt.expired.bundle"), SIGNER),
    signatureFailure,
  );
});

test("a bundle that cannot be read is an absent signature, not a broken one", async () => {
  for (const bundle of [Buffer.from(""), Buffer.from("{}"), fixture(BUNDLE_ASSET).subarray(0, 200)]) {
    const error = await rejection(verifyManifest(fixture(MANIFEST_ASSET), bundle, SIGNER));
    assert.ok(error instanceof UnsignedReleaseError);
    assert.match(error.message, /could not be read/);
  }
});

test("a root of trust that will not load is an absent signature", async () => {
  const missing = new ReleaseSigner({
    issuer: identity.issuer,
    project: identity.project,
    trustedRootPath: join(FIXTURES, "no-such-trusted-root.json"),
  });
  const error = await rejection(
    verifyManifest(fixture(MANIFEST_ASSET), fixture(BUNDLE_ASSET), missing),
  );
  assert.ok(error instanceof UnsignedReleaseError);
  assert.match(error.message, /root of trust/);
});

test("an asset the signed manifest does not cover is not vouched for by it", async () => {
  const error = await rejection(
    verifiedManifestDigest(
      fixture(MANIFEST_ASSET),
      fixture(BUNDLE_ASSET),
      "sysml-grpc-windows-amd64.exe",
      SIGNER,
    ),
  );
  assert.ok(error instanceof UnsignedReleaseError);
  assert.match(error.message, /lists no SHA-256 digest/);
});

test("the manifest is read as sha256sum writes it", () => {
  const manifest = Buffer.from(
    ["ab".repeat(32) + "  sysml-grpc-linux-amd64", "cd".repeat(32) + " *sysml-grpc-linux-arm64", "", "not a checksum line"].join("\n") + "\n",
  );
  assert.equal(manifestDigest(manifest, "sysml-grpc-linux-amd64"), "ab".repeat(32));
  // A binary-mode entry is the same digest, marked with a star.
  assert.equal(manifestDigest(manifest, "sysml-grpc-linux-arm64"), "cd".repeat(32));
  assert.equal(manifestDigest(manifest, "sysml-grpc-darwin-arm64"), undefined);
  assert.equal(manifestDigest(Buffer.from("nonsense  sysml-grpc-linux-amd64\n"), "sysml-grpc-linux-amd64"), undefined);
});

test("a signed release installs with no pin, and without any opt-in", async (t) => {
  const release = await serveRelease(signed());
  t.after(() => release.close());
  const options = downloadOptions(release);

  assert.equal(await signedManifestDigest(VERSION, ASSET, options), DIGEST);
  const path = await downloadBinary(options);
  assert.deepEqual(await readFile(path), BODY);
  assert.equal(process.env[ALLOW_UNPINNED_ENV], undefined);
  // The cache records a signed release like any other.
  assert.deepEqual(JSON.parse(readFileSync(metadataPath(options), "utf8")), {
    version: VERSION,
    sha256: DIGEST,
    repo: REPO,
  });
  assert.equal(await cachedRelease(options), VERSION);
});

test("a release with no manifest, or no bundle, is refused as an unpinned one", async (t) => {
  for (const missing of [MANIFEST_ASSET, BUNDLE_ASSET]) {
    const release = await serveRelease({ ...signed(), ...without(missing) });
    t.after(() => release.close());
    const options = downloadOptions(release);

    const error = await rejection(downloadBinary(options));
    assert.ok(error instanceof UnpinnedReleaseError);
    assert.match(error.message, /pins no SHA-256 digest/);
    assert.match(error.message, new RegExp(`no readable ${missing.replaceAll(".", "\\.")}`));
    await assert.rejects(readFile(cachedBinaryPath(options)));
  }
});

test("a signature that fails is not the same as one that is missing, and no opt-in bypasses it", async (t) => {
  const release = await serveRelease({
    ...signed(),
    bundle: fixture("SHA256SUMS.txt.other-identity.bundle"),
  });
  t.after(() => release.close());
  process.env[ALLOW_UNPINNED_ENV] = "1";
  const options = downloadOptions(release);

  const error = await rejection(downloadBinary(options));
  assert.ok(error instanceof ChecksumMismatchError);
  assert.ok(!(error instanceof UnpinnedReleaseError));
  await assert.rejects(readFile(cachedBinaryPath(options)));
});

test("a signed manifest that contradicts a pin is a hard failure", async (t) => {
  const release = await serveRelease(signed());
  t.after(() => release.close());
  const options: DownloadOptions = {
    ...downloadOptions(release),
    pinnedDigests: { [REPO]: { [VERSION]: { [ASSET]: "ab".repeat(32) } } },
  };

  // The pin is what the download must hash to, so it fails on the sidecar it
  // disagrees with before the manifest is even fetched.
  const error = await rejection(downloadBinary(options));
  assert.ok(error instanceof ChecksumMismatchError);
  assert.ok(!(error instanceof UnpinnedReleaseError));
});

test("the signed digest is what a download is verified against, not the served one", async (t) => {
  // A signed manifest and a sidecar that disagree: only the signed digest counts,
  // and the body matches it, so the sidecar's claim changes nothing.
  const release = await serveRelease({ ...signed(), checksum: "ab".repeat(32) });
  t.after(() => release.close());
  const options = downloadOptions(release);
  assert.deepEqual(await readFile(await downloadBinary(options)), BODY);

  // With a body the signed manifest does not cover, the download is refused.
  const wrong = await serveRelease({ ...signed(), binary: Buffer.from("another build\n") });
  t.after(() => wrong.close());
  await assert.rejects(downloadBinary(downloadOptions(wrong)), ChecksumMismatchError);
});

test("a signed manifest covers an asset by name, under whatever tag publishes it", async (t) => {
  // The manifest is read from the release being installed, so what it binds is the
  // asset's name to its bytes; the tag is which release's manifest was fetched.
  const release = await serveRelease({ ...signed(), version: "v9.9.8" });
  t.after(() => release.close());
  const options: DownloadOptions = { ...downloadOptions(release), version: "v9.9.8" };
  assert.deepEqual(await readFile(await downloadBinary(options)), BODY);
});

/** The recorded release, served whole. */
function signed(): Release {
  return {
    version: VERSION,
    repo: REPO,
    asset: ASSET,
    binary: BODY,
    checksum: DIGEST,
    manifest: fixture(MANIFEST_ASSET),
    bundle: fixture(BUNDLE_ASSET),
  };
}

function without(asset: string): Partial<Release> {
  return asset === MANIFEST_ASSET ? { manifest: undefined } : { bundle: undefined };
}

/** Options for a download of the recorded release, with nothing pinned for it. */
function downloadOptions(release: { url: string; apiUrl: string }): DownloadOptions {
  return {
    version: VERSION,
    githubRepo: REPO,
    releasesBaseUrl: release.url,
    apiBaseUrl: release.apiUrl,
    cacheDir: mkdtempSync(join(tmpdir(), "signing-cache-")),
    platform: "linux",
    arch: "x64",
    pinnedDigests: {},
    signer: SIGNER,
    warn: () => undefined,
  };
}

function fixture(name: string): Buffer {
  return readFileSync(join(FIXTURES, name));
}

/** A signature that was checked and did not verify, which no opt-in bypasses. */
function signatureFailure(error: unknown): boolean {
  assert.ok(error instanceof ChecksumMismatchError);
  assert.ok(!(error instanceof UnpinnedReleaseError));
  assert.match(error.message, /does not verify/);
  return true;
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
