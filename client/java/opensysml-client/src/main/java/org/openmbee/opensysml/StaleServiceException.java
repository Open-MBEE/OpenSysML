package org.openmbee.opensysml;

import java.util.Objects;

/**
 * The service reached is not the release asked for. A service this client did not start is
 * reported rather than stopped, since the client cannot assume the process is its own.
 */
public class StaleServiceException extends ServiceStartException {

  private static final long serialVersionUID = 1L;

  private final String address;
  private final String reason;
  private final String remedy;
  private final String serviceVersion;

  /**
   * Creates the exception.
   *
   * @param address the address the mismatched service is listening on
   * @param reason how it differs from the service asked for
   * @param remedy what to do about it
   * @param serviceVersion the version it reported, empty when it could not say
   */
  public StaleServiceException(
      String address, String reason, String remedy, String serviceVersion) {
    super(
        "the sysml-grpc service already listening on "
            + address
            + " is not the one this client asked for: "
            + reason
            + ".\n  service: sysml-grpc "
            + (serviceVersion.isEmpty() ? "(version unknown)" : serviceVersion)
            + " at "
            + address
            + "\n  fix:     "
            + remedy);
    this.address = Objects.requireNonNull(address, "address");
    this.reason = Objects.requireNonNull(reason, "reason");
    this.remedy = Objects.requireNonNull(remedy, "remedy");
    this.serviceVersion = Objects.requireNonNull(serviceVersion, "serviceVersion");
  }

  /**
   * The address the mismatched service is listening on.
   *
   * @return host:port
   */
  public String address() {
    return address;
  }

  /**
   * How the service differs from the one asked for.
   *
   * @return the reason
   */
  public String reason() {
    return reason;
  }

  /**
   * What to do about it.
   *
   * @return the remedy
   */
  public String remedy() {
    return remedy;
  }

  /**
   * The version the service reported.
   *
   * @return the version, empty when it did not answer {@code GetServerInfo}
   */
  public String serviceVersion() {
    return serviceVersion;
  }
}
