package org.openmbee.opensysml;

/**
 * The service could not be reached, or the answer could not be read: a failure of the transport
 * rather than of the service.
 *
 * <p>Reported as {@link StatusCode#UNAVAILABLE}, which is what the Connect protocol maps a failed
 * connection to, except for a call that outlived its own timeout: that is
 * {@link StatusCode#DEADLINE_EXCEEDED}, so a caller retrying an unreachable service does not retry
 * a call the service may well be running.
 */
public class TransportException extends ServiceException {

  private static final long serialVersionUID = 1L;

  /**
   * Creates a transport exception reported as {@link StatusCode#UNAVAILABLE}.
   *
   * @param message what failed
   * @param cause the underlying I/O failure
   */
  public TransportException(String message, Throwable cause) {
    this(StatusCode.UNAVAILABLE, message, cause);
  }

  /**
   * Creates a transport exception of a status.
   *
   * @param status the status the failure maps to
   * @param message what failed
   * @param cause the underlying failure, or {@code null} when there is none
   */
  public TransportException(StatusCode status, String message, Throwable cause) {
    super(status, message, cause);
  }
}
