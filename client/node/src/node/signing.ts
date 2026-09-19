// Verification of the signed checksum manifest a release publishes. A core release
// signs its SHA256SUMS.txt in CI with cosign keyless, using the release pipeline's
// CircleCI OIDC identity, and publishes the sigstore bundle beside it. A bundle
// that verifies against the identity pinned here says the manifest came from that
// pipeline, so a release published after this client can still be installed. A
// manifest that does not verify is refused exactly as an unpinned release is.

import { readFile } from "node:fs/promises";
import { ManifestSignatureError, UnsignedReleaseError } from "../core/errors.js";

/** Manifest of every published artifact's digest, and its sigstore bundle. */
export const MANIFEST_ASSET = "SHA256SUMS.txt";
export const BUNDLE_ASSET = `${MANIFEST_ASSET}.bundle`;

/** How long the root of trust may take to load, matching the download timeout. */
const TRUST_ROOT_TIMEOUT_MS = 15_000;

// A CircleCI signing certificate's subject names the pipeline definition that
// produced it, under the project it belongs to.
const PIPELINE_DEFINITIONS = "/pipeline-definitions/";
const DEFINITION_ID = "[0-9a-f]{8}(-[0-9a-f]{4}){3}-[0-9a-f]{12}";

const SHA256 = /^[0-9a-f]{64}$/;

/** The sigstore packages this module verifies with, imported on demand. */
interface Sigstore {
  bundle: typeof import("@sigstore/bundle");
  specs: typeof import("@sigstore/protobuf-specs");
  tuf: typeof import("@sigstore/tuf");
  verify: typeof import("@sigstore/verify");
}

/** The identity a release manifest's signature must carry. */
export class ReleaseSigner {
  /** OIDC issuer the certificate must have been issued for. */
  readonly issuer: string;
  /** CircleCI project API URL the certificate subject must belong to. */
  readonly project: string;
  /** Pipeline definition the subject must name exactly, or any of the project. */
  readonly definition: string | undefined;
  /** Sigstore trusted root to verify against, or Sigstore's production instance. */
  readonly trustedRootPath: string | undefined;

  constructor(init: {
    issuer: string;
    project: string;
    definition?: string;
    trustedRootPath?: string;
  }) {
    this.issuer = init.issuer;
    this.project = init.project;
    this.definition = init.definition;
    this.trustedRootPath = init.trustedRootPath;
  }

  /** Certificate subject required, or undefined when any definition is accepted. */
  get subject(): string | undefined {
    if (this.definition === undefined) {
      return undefined;
    }
    return this.project + PIPELINE_DEFINITIONS + this.definition;
  }

  /** The identity, for an error message. */
  describe(): string {
    const subject = this.subject ?? `${this.project}${PIPELINE_DEFINITIONS}*`;
    return `issuer ${this.issuer}, subject ${subject}`;
  }

  /**
   * The verification policy for this identity. The subject pattern is anchored,
   * since sigstore matches it as an unanchored regular expression.
   */
  policy(): import("@sigstore/verify").VerificationPolicy {
    const subject = this.subject;
    const pattern =
      subject === undefined
        ? // No definition is pinned, so any definition of the project is accepted:
          // no unauthenticated API publishes the identifier to pin.
          escapeForRegExp(this.project + PIPELINE_DEFINITIONS) + DEFINITION_ID
        : escapeForRegExp(subject);
    return {
      extensions: { issuer: this.issuer },
      subjectAlternativeName: `^${pattern}$`,
    };
  }

  /** A verifier holding the root of trust this signer's certificates chain to. */
  async verifier(sigstore: Sigstore): Promise<import("@sigstore/verify").Verifier> {
    const root =
      this.trustedRootPath === undefined
        ? await sigstore.tuf.getTrustedRoot({ timeout: TRUST_ROOT_TIMEOUT_MS })
        : sigstore.specs.TrustedRoot.fromJSON(
            JSON.parse(await readFile(this.trustedRootPath, "utf8")),
          );
    return new sigstore.verify.Verifier(sigstore.verify.toTrustMaterial(root));
  }
}

/**
 * Identity each repository's release manifest must be signed with: the CircleCI OIDC
 * issuer of its organization and its project. A repository absent here signs nothing.
 */
export const SIGNED_MANIFEST_SIGNERS: Readonly<Record<string, ReleaseSigner>> = {
  "Open-MBEE/OpenSysML": new ReleaseSigner({
    issuer: "https://oidc.circleci.com/org/1169df8b-0b59-400f-82d2-c9d8e98bdb62",
    project: "https://circleci.com/api/v2/projects/eeb0dddd-237f-4f02-9e51-8e24caef589d",
  }),
};

/** The signer whose signature a repository's release manifest must carry. */
export function signerFor(githubRepo: string): ReleaseSigner | undefined {
  return Object.hasOwn(SIGNED_MANIFEST_SIGNERS, githubRepo) ? SIGNED_MANIFEST_SIGNERS[githubRepo] : undefined;
}

/** The digest a checksum manifest lists for an asset, as sha256sum writes them. */
export function manifestDigest(manifest: Buffer, asset: string): string | undefined {
  for (const line of manifest.toString("utf8").split("\n")) {
    const fields = line.split(/\s+/).filter((field) => field !== "");
    if (fields.length !== 2) {
      continue;
    }
    const [digest, name] = fields as [string, string];
    // sha256sum marks a file it read in binary mode with a leading '*'.
    if (name.replace(/^\*+/, "") !== asset) {
      continue;
    }
    const lowered = digest.toLowerCase();
    return SHA256.test(lowered) ? lowered : undefined;
  }
  return undefined;
}

/**
 * Verify a manifest's sigstore bundle against the signer expected: nothing checked at
 * all is an UnsignedReleaseError, a signature that fails a ManifestSignatureError.
 */
export async function verifyManifest(
  manifest: Buffer,
  bundle: Buffer,
  signer: ReleaseSigner,
): Promise<void> {
  const sigstore = await loadSigstore();

  let read: import("@sigstore/bundle").Bundle;
  try {
    read = sigstore.bundle.bundleFromJSON(JSON.parse(bundle.toString("utf8")));
  } catch (cause) {
    // A truncated download looks like this too, so it is an absent signature
    // rather than evidence of a tampered one.
    throw new UnsignedReleaseError(
      `the sigstore bundle published for ${MANIFEST_ASSET} could not be read ` +
        `(${describe(cause)}), so the manifest was not verified.`,
      { cause },
    );
  }

  let verifier: import("@sigstore/verify").Verifier;
  try {
    verifier = await signer.verifier(sigstore);
  } catch (cause) {
    throw new UnsignedReleaseError(
      `this client could not load the sigstore root of trust needed to verify ` +
        `${MANIFEST_ASSET} (${describe(cause)}), so the manifest was not verified.`,
      { cause },
    );
  }

  try {
    verifier.verify(sigstore.verify.toSignedEntity(read, manifest), signer.policy());
  } catch (cause) {
    throw new ManifestSignatureError(
      `the signature on ${MANIFEST_ASSET} does not verify against the release ` +
        `pipeline of this repository (${signer.describe()}): ${describe(cause)}. The ` +
        `manifest, the signature, or both were replaced; nothing was installed.`,
      { cause },
    );
  }
}

/** The digest a signed manifest lists for an asset, once its signature verified. */
export async function verifiedManifestDigest(
  manifest: Buffer,
  bundle: Buffer,
  asset: string,
  signer: ReleaseSigner,
): Promise<string> {
  await verifyManifest(manifest, bundle, signer);
  const digest = manifestDigest(manifest, asset);
  if (digest === undefined) {
    throw new UnsignedReleaseError(
      `the signed ${MANIFEST_ASSET} of this release lists no SHA-256 digest for ` +
        `${asset}, so that asset is not covered by the signature.`,
    );
  }
  return digest;
}

/** The verifier packages, or a refusal explaining that they are not installed. */
async function loadSigstore(): Promise<Sigstore> {
  try {
    const [bundle, specs, tuf, verify] = await Promise.all([
      import("@sigstore/bundle"),
      import("@sigstore/protobuf-specs"),
      import("@sigstore/tuf"),
      import("@sigstore/verify"),
    ]);
    return { bundle, specs, tuf, verify };
  } catch (cause) {
    throw new UnsignedReleaseError(
      `this client cannot verify the signature on ${MANIFEST_ASSET}: ` +
        `${describe(cause)}. Install its optional sigstore dependencies, or ask for ` +
        `a release it pins a digest for with version.`,
      { cause },
    );
  }
}

function describe(cause: unknown): string {
  return cause instanceof Error ? `${cause.name}: ${cause.message}` : String(cause);
}

function escapeForRegExp(literal: string): string {
  return literal.replace(/[.*+?^${}()|[\]\\/]/g, String.raw`\$&`);
}
