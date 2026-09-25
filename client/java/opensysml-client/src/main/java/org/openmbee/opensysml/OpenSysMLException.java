package org.openmbee.opensysml;

/**
 * Base of every failure this client reports.
 *
 * <p>The exception policy is one rule: everything this client throws is unchecked and extends this
 * class. A host application embedding the client cannot usually recover from a refused call, a
 * service that will not start or a model that does not parse at the call site, and forcing a
 * {@code catch} on each of ~10 call sites of an Eclipse plugin buys nothing; where a caller does
 * want to react, it catches this class or one subclass of it. Absent values are reported as
 * {@link java.util.Optional}, never as an exception.
 */
public class OpenSysMLException extends RuntimeException {

  private static final long serialVersionUID = 1L;

  /**
   * Creates an exception.
   *
   * @param message what failed
   */
  public OpenSysMLException(String message) {
    super(message);
  }

  /**
   * Creates an exception with a cause.
   *
   * @param message what failed
   * @param cause the underlying failure
   */
  public OpenSysMLException(String message, Throwable cause) {
    super(message, cause);
  }
}
