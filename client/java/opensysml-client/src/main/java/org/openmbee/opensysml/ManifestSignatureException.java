package org.openmbee.opensysml;

/**
 * The signature on a release's checksum manifest does not verify.
 *
 * <p>A bundle was published and read, and it does not attest that this release pipeline produced
 * this manifest — another signer, an expired certificate, or a manifest changed after signing.
 * That is evidence, not absence, so it is a mismatch: no cached binary answers for it.
 */
public class ManifestSignatureException extends ChecksumMismatchException {

  private static final long serialVersionUID = 1L;

  /**
   * Creates a signature failure.
   *
   * @param message what did not verify
   */
  public ManifestSignatureException(String message) {
    super(message);
  }
}
