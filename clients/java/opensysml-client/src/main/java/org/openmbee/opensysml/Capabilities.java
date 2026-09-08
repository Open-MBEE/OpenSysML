package org.openmbee.opensysml;

import java.util.Objects;
import java.util.Set;

/**
 * What a running service says it can do.
 *
 * <p>Negotiation is on capability names, never on {@link #serviceVersion()}: the version is
 * informational ({@code "dev"} for an unreleased build) and versions of forks are not comparable.
 */
public final class Capabilities {

  /** {@code SymbolInfo} carries type facts, multiplicity and specializations. */
  public static final String TYPE_FACTS = "type_facts";

  /** The {@code Convert} RPC writes a model back out. */
  public static final String CONVERT = "convert";

  /** The verification RPCs answer whether constraints and requirements hold. */
  public static final String VERIFICATION = "verification";

  /** A verification, satisfaction or analysis response carries what a case body answered. */
  public static final String VERIFICATION_VERDICTS = "verification_verdicts";

  /** The {@code Query} RPC evaluates a SysML v2 API and Services Query. */
  public static final String QUERY = "query";

  /** The {@code Query} RPC evaluates OSLC Query text. */
  public static final String OSLC_QUERY = "oslc_query";

  /** An enumeration literal travels as itself rather than as a null. */
  public static final String ENUM_VALUES = "enum_values";

  /** {@code Evaluate} honours a subject symbol instead of ignoring it. */
  public static final String EVALUATE_SUBJECT = "evaluate_subject";

  /** A valueless feature of a value type is reported as unset. */
  public static final String UNSET_VALUE = "unset_value";

  /** A complex number travels as itself rather than as an unsupported null. */
  public static final String COMPLEX_VALUES = "complex_values";

  /** An array, a vector and a vector quantity travel as themselves rather than as unsupported nulls. */
  public static final String STRUCTURED_VALUES = "structured_values";

  /** A bare measurement unit ({@code SI::m}, {@code m / s}) travels as itself rather than as an unsupported null. */
  public static final String MEASUREMENT_REFS = "measurement_refs";

  /** The {@code ApplyEdits} RPC edits a parsed model's own source. */
  public static final String APPLY_EDITS = "apply_edits";

  /** {@code SymbolInfo} carries the attributes of a symbol rather than none. */
  public static final String SYMBOL_ATTRIBUTES = "symbol_attributes";

  /** {@code ApplyEdits} can add members and delete declarations. */
  public static final String AUTHORING = "authoring";

  /** Inline content may name the notation it is written in. */
  public static final String INLINE_LANGUAGE = "inline_language";

  /** A parse can judge the source as conforming SysML v2. */
  public static final String STRICT_CONFORMANCE = "strict_conformance";

  /** An instance carries what it holds for each feature of its type. */
  public static final String FEATURE_VALUES = "feature_values";

  private final String serviceVersion;
  private final Set<String> names;

  /**
   * Creates a capability set.
   *
   * @param serviceVersion build version the service reported
   * @param names capability names the service advertised
   */
  public Capabilities(String serviceVersion, Set<String> names) {
    this.serviceVersion = Objects.requireNonNull(serviceVersion, "serviceVersion");
    this.names = Set.copyOf(Objects.requireNonNull(names, "names"));
  }

  /**
   * Build version of the service, informational only.
   *
   * @return the version string the service reported
   */
  public String serviceVersion() {
    return serviceVersion;
  }

  /**
   * Every capability name the service advertised.
   *
   * @return an unmodifiable set of names
   */
  public Set<String> names() {
    return names;
  }

  /**
   * Whether the service advertises a capability.
   *
   * @param capability the capability name
   * @return {@code true} when it is advertised
   */
  public boolean has(String capability) {
    return names.contains(Objects.requireNonNull(capability, "capability"));
  }

  /**
   * Requires a capability, failing when the service does not advertise it.
   *
   * @param capability the capability name
   * @throws CapabilityException if the service does not advertise it
   */
  public void require(String capability) {
    if (!has(capability)) {
      throw new CapabilityException(
          capability,
          "the service does not advertise the "
              + capability
              + " capability; it advertises "
              + names);
    }
  }

  @Override
  public boolean equals(Object other) {
    return other instanceof Capabilities capabilities
        && serviceVersion.equals(capabilities.serviceVersion)
        && names.equals(capabilities.names);
  }

  @Override
  public int hashCode() {
    return Objects.hash(serviceVersion, names);
  }

  @Override
  public String toString() {
    return "Capabilities[version=" + serviceVersion + ", names=" + names + "]";
  }
}
