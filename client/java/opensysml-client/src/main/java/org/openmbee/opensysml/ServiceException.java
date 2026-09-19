package org.openmbee.opensysml;

import java.util.Objects;

/**
 * A call the service refused: it carries a status rather than an answer.
 *
 * <p>Distinct from {@link ModelException}, which is a failure reported inside an answer the service
 * did give.
 */
public class ServiceException extends OpenSysMLException {

  private static final long serialVersionUID = 1L;

  private final StatusCode status;
  private final String serviceMessage;

  /**
   * Creates a refused-call exception.
   *
   * @param status the status the call was refused with
   * @param message what the service said
   */
  public ServiceException(StatusCode status, String message) {
    super(status + ": " + message);
    this.status = Objects.requireNonNull(status, "status");
    this.serviceMessage = Objects.requireNonNull(message, "message");
  }

  /**
   * Creates a refused-call exception with a cause.
   *
   * @param status the status the call was refused with
   * @param message what the service said
   * @param cause the underlying failure
   */
  public ServiceException(StatusCode status, String message, Throwable cause) {
    super(status + ": " + message, cause);
    this.status = Objects.requireNonNull(status, "status");
    this.serviceMessage = Objects.requireNonNull(message, "message");
  }

  /**
   * What the service said, without the status this exception's message prefixes it with.
   *
   * @return the status message the service sent
   */
  public String serviceMessage() {
    return serviceMessage;
  }

  /**
   * The status the call was refused with.
   *
   * @return the status, never {@link StatusCode#OK}
   */
  public StatusCode status() {
    return status;
  }
}
