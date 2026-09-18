package org.openmbee.opensysml;

import java.util.Objects;

/**
 * The service does not advertise a capability the call needs.
 *
 * <p>Thrown before the call is made: the service does not answer {@code UNIMPLEMENTED} for a
 * capability it lacks, so the advertised list is the only reliable answer.
 */
public class CapabilityException extends OpenSysMLException {

  private static final long serialVersionUID = 1L;

  private final String capability;

  /**
   * Creates a capability exception.
   *
   * @param capability the capability name that is missing
   * @param message what needed it
   */
  public CapabilityException(String capability, String message) {
    super(message);
    this.capability = Objects.requireNonNull(capability, "capability");
  }

  /**
   * The capability the service does not advertise.
   *
   * @return the capability name
   */
  public String capability() {
    return capability;
  }
}
