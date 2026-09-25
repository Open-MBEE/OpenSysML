package org.openmbee.opensysml;

/**
 * A release publishes no signature this client could check.
 *
 * <p>An old release published before the pipeline signed its checksum manifest, one whose bundle
 * asset is missing or unreadable, or a classpath without the sigstore verifier: no signature was
 * checked, so the release is one this client cannot vouch for, exactly as an unpinned one is.
 */
public class UnsignedReleaseException extends UnpinnedReleaseException {

  private static final long serialVersionUID = 1L;

  /**
   * Creates a refusal.
   *
   * @param message why nothing was verified
   */
  public UnsignedReleaseException(String message) {
    super(message);
  }
}
