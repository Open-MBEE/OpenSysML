package org.openmbee.opensysml.internal;

import org.openmbee.opensysml.UnsignedReleaseException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Path;
import java.util.Locale;
import java.util.Map;
import java.util.Optional;
import java.util.regex.Pattern;

/**
 * The signed checksum manifest a release publishes, and the identity its signature must carry.
 *
 * <p>A core release signs its {@code SHA256SUMS.txt} in CI with cosign keyless, using the release
 * pipeline's CircleCI OIDC identity, and publishes the sigstore bundle beside it. A bundle that
 * verifies against the identity pinned here says the manifest was produced by that pipeline, so a
 * release published after this client can still be installed. A manifest that does not verify is
 * refused exactly as an unpinned release is.
 */
public final class SignedManifest {

  /** Manifest of every published artifact's digest, and its sigstore bundle. */
  public static final String MANIFEST_ASSET = "SHA256SUMS.txt";

  /** The sigstore bundle published beside {@link #MANIFEST_ASSET}. */
  public static final String BUNDLE_ASSET = MANIFEST_ASSET + ".bundle";

  private static final Pattern SHA256 = Pattern.compile("[0-9a-f]{64}");
  private static final Pattern LINE = Pattern.compile("\\R");
  private static final Pattern FIELD = Pattern.compile("\\s+");
  private static final Pattern BINARY_MARK = Pattern.compile("^\\*+");

  /**
   * A CircleCI signing certificate's subject names the pipeline definition that produced it, whose
   * identifier no unauthenticated API publishes, so this pins the project it must belong to.
   */
  private static final String DEFINITION_ID = "[0-9a-f]{8}(-[0-9a-f]{4}){3}-[0-9a-f]{12}";

  /**
   * Identity the signature on each repository's release manifest must carry. A repository absent
   * here signs nothing this client can check, so its unpinned releases are refused.
   */
  private static final Map<String, ReleaseSigner> SIGNERS =
      Map.of(
          "Open-MBEE/OpenSysML",
          new ReleaseSigner(
              "https://oidc.circleci.com/org/1169df8b-0b59-400f-82d2-c9d8e98bdb62",
              "https://circleci.com/api/v2/projects/eeb0dddd-237f-4f02-9e51-8e24caef589d"));

  private SignedManifest() {}

  /**
   * The signer whose signature a repository's release manifest must carry.
   *
   * @param githubRepo repository (owner/repo)
   * @return the signer, or empty when the repository publishes no signature this client knows how
   *     to check
   */
  public static Optional<ReleaseSigner> signerFor(String githubRepo) {
    return Optional.ofNullable(SIGNERS.get(githubRepo));
  }

  /**
   * The digest a checksum manifest lists for an asset.
   *
   * @param manifest manifest content, as {@code sha256sum} writes it
   * @param asset asset name (e.g. {@code sysml-grpc-linux-amd64})
   * @return the hex digest, or empty when the manifest lists no well-formed digest for it
   */
  public static Optional<String> digestFor(byte[] manifest, String asset) {
    for (String line : LINE.split(new String(manifest, StandardCharsets.UTF_8))) {
      String[] fields = FIELD.split(line.trim());
      // sha256sum marks a file it read in binary mode with a leading '*'.
      if (fields.length == 2 && BINARY_MARK.matcher(fields[1]).replaceFirst("").equals(asset)) {
        String digest = fields[0].toLowerCase(Locale.ROOT);
        return SHA256.matcher(digest).matches() ? Optional.of(digest) : Optional.empty();
      }
    }
    return Optional.empty();
  }

  /**
   * The digest a signed manifest lists for an asset, once its signature verified.
   *
   * @param manifest manifest content downloaded from the release
   * @param bundle sigstore bundle downloaded beside it
   * @param asset asset name (e.g. {@code sysml-grpc-linux-amd64})
   * @param signer identity the signature must carry
   * @param trustedRoot a sigstore trusted root to verify against, or {@code null} for Sigstore's
   *     production instance
   * @return the hex digest, from a manifest signed by that pipeline
   * @throws UnsignedReleaseException if nothing was verified, or the verified manifest lists no
   *     digest for the asset
   * @throws org.openmbee.opensysml.ManifestSignatureException if the signature does not verify
   */
  public static String verifiedDigest(
      byte[] manifest, byte[] bundle, String asset, ReleaseSigner signer, Path trustedRoot) {
    try {
      SigstoreVerifier.verify(manifest, bundle, signer, trustedRoot);
    } catch (NoClassDefFoundError e) {
      // A classpath the verifier was excluded from refuses to verify rather than failing to load.
      throw new UnsignedReleaseException(
          "this client cannot verify the signature on "
              + MANIFEST_ASSET
              + " ("
              + e
              + "). Add the dev.sigstore:sigstore-java dependency, or ask for a release this "
              + "client pins a digest for with ConnectionOptions.downloadVersion().");
    }
    return digestFor(manifest, asset)
        .orElseThrow(
            () ->
                new UnsignedReleaseException(
                    "the signed "
                        + MANIFEST_ASSET
                        + " of this release lists no SHA-256 digest for "
                        + asset
                        + ", so that asset is not covered by the signature."));
  }

  /**
   * The identity a release manifest's signature must carry: any pipeline definition of one
   * CircleCI project, issued by that organization's OIDC issuer.
   *
   * @param issuer OIDC issuer the certificate must have been issued for
   * @param project CircleCI project API URL the subject must belong to
   */
  public record ReleaseSigner(String issuer, String project) {

    /**
     * The subject the signing certificate must carry, as a fully anchored regular expression.
     *
     * @return a pattern accepting any pipeline definition of {@link #project()}
     */
    public String subjectPattern() {
      return Pattern.quote(project + "/pipeline-definitions/") + DEFINITION_ID;
    }

    /**
     * The identity, for an error message.
     *
     * @return issuer and subject required
     */
    public String describe() {
      return "issuer " + issuer + ", subject " + project + "/pipeline-definitions/*";
    }
  }
}
