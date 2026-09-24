package org.openmbee.opensysml;

import java.util.Locale;

/**
 * The canonical status a refused call carries, named as gRPC and the Connect protocol name it.
 *
 * <p>A call the service answered has status {@link #OK} even when the answer reports a failure in
 * band; that failure is a {@link ModelException}, not a status.
 */
public enum StatusCode {
  /** The call was answered. */
  OK,
  /** The call was cancelled. */
  CANCELLED,
  /** A failure the service did not classify. */
  UNKNOWN,
  /** The request was wrong. */
  INVALID_ARGUMENT,
  /** The call outlived its deadline. */
  DEADLINE_EXCEEDED,
  /** What the request named does not exist. */
  NOT_FOUND,
  /** What the request would create exists already. */
  ALREADY_EXISTS,
  /** The caller may not do this. */
  PERMISSION_DENIED,
  /** A resource is exhausted. */
  RESOURCE_EXHAUSTED,
  /** The service is not in a state where this call can succeed. */
  FAILED_PRECONDITION,
  /** The call was aborted. */
  ABORTED,
  /** An argument is outside the valid range. */
  OUT_OF_RANGE,
  /** This build of the service does not implement the call. */
  UNIMPLEMENTED,
  /** The service failed internally. */
  INTERNAL,
  /** The service could not be reached, or is not serving. */
  UNAVAILABLE,
  /** Data was lost or corrupted. */
  DATA_LOSS,
  /** The caller is not authenticated. */
  UNAUTHENTICATED;

  /**
   * The status a Connect error body names.
   *
   * @param connectCode the {@code code} of a Connect error body ({@code "invalid_argument"})
   * @return the matching status, or {@link #UNKNOWN} for a code this release does not know
   */
  public static StatusCode fromConnectCode(String connectCode) {
    if (connectCode == null || connectCode.isEmpty()) {
      return UNKNOWN;
    }
    try {
      return valueOf(connectCode.toUpperCase(Locale.ROOT));
    } catch (IllegalArgumentException e) {
      return UNKNOWN;
    }
  }

  /**
   * The status Connect maps an HTTP status to, for a response carrying no readable error body.
   *
   * @param httpStatus the HTTP status of the response
   * @return the matching status
   */
  public static StatusCode fromHttpStatus(int httpStatus) {
    return switch (httpStatus) {
      case 400 -> INTERNAL;
      case 401 -> UNAUTHENTICATED;
      case 403 -> PERMISSION_DENIED;
      case 404 -> UNIMPLEMENTED;
      case 429 -> UNAVAILABLE;
      case 502, 503, 504 -> UNAVAILABLE;
      default -> UNKNOWN;
    };
  }

  /**
   * This status as the Connect protocol spells it.
   *
   * @return the lower-case Connect code name
   */
  public String connectCode() {
    return name().toLowerCase(Locale.ROOT);
  }
}
