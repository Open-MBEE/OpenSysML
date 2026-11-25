package org.openmbee.opensysml.repository;

/** A SysML v2 API request that could not be made, or that the server refused. */
public class ApiException extends RuntimeException {
  private static final long serialVersionUID = 1L;
  private final int status;

  public ApiException(String message) {
    this(message, 0, null);
  }

  public ApiException(String message, Throwable cause) {
    this(message, 0, cause);
  }

  public ApiException(String message, int status, Throwable cause) {
    super(message, cause);
    this.status = status;
  }

  /** The HTTP status the server answered with, or 0 when the failure was not an answer. */
  public int status() {
    return status;
  }
}
