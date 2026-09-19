package org.openmbee.opensysml;

/**
 * A private service could not be started: no binary was found, it did not report an address, or it
 * exited while starting.
 */
public class ServiceStartException extends OpenSysMLException {

  private static final long serialVersionUID = 1L;

  /**
   * Creates a start exception.
   *
   * @param message what failed
   */
  public ServiceStartException(String message) {
    super(message);
  }

  /**
   * Creates a start exception with a cause.
   *
   * @param message what failed
   * @param cause the underlying failure
   */
  public ServiceStartException(String message, Throwable cause) {
    super(message, cause);
  }
}
