package org.openmbee.opensysml;

/**
 * This client pins no digest for the release being downloaded, and nothing else vouched for it.
 *
 * <p>Nothing contradicts anything here: the release is simply not one this client can verify, so a
 * working cached binary may still be used because no download is under suspicion.
 */
public class UnpinnedReleaseException extends ChecksumMismatchException {

  private static final long serialVersionUID = 1L;

  /**
   * Creates a refusal.
   *
   * @param message what is not pinned, and how to proceed
   */
  public UnpinnedReleaseException(String message) {
    super(message);
  }
}
