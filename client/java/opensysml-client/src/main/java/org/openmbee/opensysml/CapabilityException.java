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
  private final String remedy;

  /**
   * Creates a capability exception.
   *
   * @param capability the capability name that is missing
   * @param message what needed it
   */
  public CapabilityException(String capability, String message) {
    this(capability, message, "");
  }

  /**
   * Creates a capability exception that says how to reach a service that has the capability.
   *
   * @param capability the capability name that is missing
   * @param message what needed it
   * @param remedy how to run a service that advertises it, or empty
   */
  public CapabilityException(String capability, String message, String remedy) {
    super(message);
    this.capability = Objects.requireNonNull(capability, "capability");
    this.remedy = Objects.requireNonNull(remedy, "remedy");
  }

  /**
   * How to run a service that advertises the capability.
   *
   * @return the remedy, or empty when none was given
   */
  public String remedy() {
    return remedy;
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
