package org.openmbee.opensysml;

import java.util.List;
import java.util.Objects;
import java.util.Optional;

/**
 * A magnitude and the measurement reference it is expressed in.
 *
 * <p>The magnitude keeps {@code Integer} and {@code Real} apart the way {@link Value} does: it is a
 * {@link Long} or a {@link Double}, never converted on the way in.
 *
 * @param magnitude the magnitude, a {@link Long} or a {@link Double}
 * @param unit the unit as written ({@code "km/h"}), or empty for one never written down
 * @param reduction what the unit reduces to, absent when the service sent none
 */
public record Quantity(Number magnitude, Optional<String> unit, Optional<UnitTerm> reduction) {

  /**
   * Creates a quantity.
   *
   * @param magnitude the magnitude, a {@link Long} or a {@link Double}
   * @param unit the unit as written, absent when unnamed
   * @param reduction the unit's reduction to base units, absent when the service sent none
   */
  public Quantity {
    Objects.requireNonNull(magnitude, "magnitude");
    Objects.requireNonNull(unit, "unit");
    Objects.requireNonNull(reduction, "reduction");
    if (!(magnitude instanceof Long) && !(magnitude instanceof Double)) {
      throw new IllegalArgumentException("magnitude must be a Long or a Double: " + magnitude);
    }
  }

  /**
   * Whether the magnitude is an {@code Integer} rather than a {@code Real}.
   *
   * @return {@code true} for an integral magnitude
   */
  public boolean isIntegral() {
    return magnitude instanceof Long;
  }

  /**
   * One base unit of a reduction, raised to an exponent.
   *
   * @param unitId FQN of the base unit ({@code "SI::m"})
   * @param exponent the exponent it is raised to
   */
  public record UnitFactor(String unitId, double exponent) {
    /**
     * Creates a unit factor.
     *
     * @param unitId FQN of the base unit, never {@code null}
     * @param exponent the exponent
     */
    public UnitFactor {
      Objects.requireNonNull(unitId, "unitId");
    }
  }

  /**
   * A unit reduced to a scale factor over base units, kept as a ratio so an exact conversion
   * stays exact.
   *
   * @param scaleNumerator numerator of the scale
   * @param scaleDenominator denominator of the scale
   * @param factors the base units of the reduction
   */
  public record UnitTerm(double scaleNumerator, double scaleDenominator, List<UnitFactor> factors) {
    /**
     * Creates a unit term, copying the factors.
     *
     * @param scaleNumerator numerator of the scale
     * @param scaleDenominator denominator of the scale
     * @param factors the base units, never {@code null}
     */
    public UnitTerm {
      factors = List.copyOf(factors);
    }
  }
}
