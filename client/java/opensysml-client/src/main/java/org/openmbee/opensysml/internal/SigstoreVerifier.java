package org.openmbee.opensysml.internal;

import dev.sigstore.KeylessVerificationException;
import dev.sigstore.KeylessVerifier;
import dev.sigstore.TrustedRootProvider;
import dev.sigstore.VerificationOptions;
import dev.sigstore.bundle.Bundle;
import dev.sigstore.bundle.BundleParseException;
import dev.sigstore.strings.RegexSyntaxException;
import dev.sigstore.strings.StringMatcher;
import org.openmbee.opensysml.ManifestSignatureException;
import org.openmbee.opensysml.UnsignedReleaseException;
import org.openmbee.opensysml.internal.SignedManifest.ReleaseSigner;
import java.io.StringReader;
import java.nio.charset.StandardCharsets;
import java.nio.file.Path;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;

/**
 * Verifies a release's sigstore bundle with {@code dev.sigstore:sigstore-java}.
 *
 * <p>Kept in one class because that dependency is optional: a classpath without it must refuse to
 * verify, not fail to load the client, so nothing outside this class names a sigstore type and
 * {@link SignedManifest} calls it inside a {@code NoClassDefFoundError} guard.
 */
final class SigstoreVerifier {

  private SigstoreVerifier() {}

  /**
   * Verifies that a bundle attests the release pipeline signed this manifest.
   *
   * @param manifest manifest content that was signed
   * @param bundle sigstore bundle published beside it
   * @param signer identity the signature must carry
   * @param trustedRoot a trusted root to verify against, or {@code null} for Sigstore's production
   *     instance
   * @throws UnsignedReleaseException if the bundle cannot be read or the root of trust cannot be
   *     loaded — no signature was checked either way
   * @throws ManifestSignatureException if the bundle was read and does not verify
   */
  static void verify(byte[] manifest, byte[] bundle, ReleaseSigner signer, Path trustedRoot) {
    Bundle read;
    try {
      read = Bundle.from(new StringReader(new String(bundle, StandardCharsets.UTF_8)));
    } catch (BundleParseException | RuntimeException e) {
      // A truncated download looks like this too, so it is an absent signature rather than
      // evidence of a tampered one.
      throw new UnsignedReleaseException(
          "the sigstore bundle published for "
              + SignedManifest.MANIFEST_ASSET
              + " could not be read ("
              + e
              + "), so the manifest was not verified.");
    }

    KeylessVerifier verifier;
    try {
      KeylessVerifier.Builder building = KeylessVerifier.builder();
      verifier =
          (trustedRoot == null
                  ? building.sigstorePublicDefaults()
                  : building.trustedRootProvider(TrustedRootProvider.from(trustedRoot)))
              .build();
    } catch (Exception e) {
      // Whatever keeps the root of trust from loading, nothing was verified.
      throw new UnsignedReleaseException(
          "this client could not load the sigstore root of trust needed to verify "
              + SignedManifest.MANIFEST_ASSET
              + " ("
              + e
              + "), so the manifest was not verified.");
    }

    VerificationOptions options;
    try {
      options =
          VerificationOptions.builder()
              .addCertificateMatchers(
                  VerificationOptions.CertificateMatcher.fulcio()
                      .issuer(StringMatcher.string(signer.issuer()))
                      .subjectAlternativeName(StringMatcher.regex(signer.subjectPattern()))
                      .build())
              .build();
    } catch (RegexSyntaxException e) {
      throw new IllegalStateException("the pinned signer identity is not a valid pattern", e);
    }
    try {
      verifier.verify(sha256(manifest), read, options);
    } catch (KeylessVerificationException | RuntimeException e) {
      throw new ManifestSignatureException(
          "the signature on "
              + SignedManifest.MANIFEST_ASSET
              + " does not verify against the release pipeline of this repository ("
              + signer.describe()
              + "): "
              + e.getMessage()
              + ". The manifest, the signature, or both were replaced; nothing was installed.");
    }
  }

  /** The digest of the signed artifact, which is what the verifier compares the bundle against. */
  private static byte[] sha256(byte[] content) {
    try {
      return MessageDigest.getInstance("SHA-256").digest(content);
    } catch (NoSuchAlgorithmException e) {
      throw new IllegalStateException("SHA-256 is required of every JVM", e);
    }
  }
}
