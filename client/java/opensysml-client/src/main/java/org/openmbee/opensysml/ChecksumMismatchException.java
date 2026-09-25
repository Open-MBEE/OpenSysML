package org.openmbee.opensysml;

/**
 * A downloaded release asset does not have the digest expected of it.
 *
 * <p>Its own class so that a download which may have been tampered with is never handled as a
 * transport failure and answered from whatever was cached before.
 */
public class ChecksumMismatchException extends ServiceStartException {

  private static final long serialVersionUID = 1L;

  /**
   * Creates a mismatch.
   *
   * @param message which digests disagree
   */
  public ChecksumMismatchException(String message) {
    super(message);
  }
}
