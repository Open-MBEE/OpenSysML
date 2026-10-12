package org.openmbee.opensysml;

import java.math.BigDecimal;
import java.math.BigInteger;
import java.util.Objects;

/**
 * An exact KerML {@code Rational}: a numerator and a positive denominator in lowest terms, so two
 * rationals are the same number exactly when they are {@link #equals equal}.
 *
 * <p>A Rational a {@code double} holds exactly crosses the wire as a {@code Real}; only one no
 * {@code double} holds, such as {@code 1/3} or {@code 1/10}, arrives as this.
 */
public final class Rational extends Number implements Comparable<Rational> {

  private static final long serialVersionUID = 1L;

  private final BigInteger numerator;
  private final BigInteger denominator;

  private Rational(BigInteger numerator, BigInteger denominator) {
    this.numerator = numerator;
    this.denominator = denominator;
  }

  /**
   * The rational {@code numerator / denominator}, reduced to lowest terms with a positive
   * denominator.
   *
   * @param numerator the numerator
   * @param denominator the denominator, nonzero
   * @return the rational
   * @throws ArithmeticException if the denominator is zero
   */
  public static Rational of(BigInteger numerator, BigInteger denominator) {
    Objects.requireNonNull(numerator, "numerator");
    Objects.requireNonNull(denominator, "denominator");
    if (denominator.signum() == 0) {
      throw new ArithmeticException("a rational's denominator is zero");
    }
    BigInteger gcd = numerator.gcd(denominator);
    if (denominator.signum() < 0) {
      gcd = gcd.negate();
    }
    return new Rational(numerator.divide(gcd), denominator.divide(gcd));
  }

  /**
   * The rational {@code numerator / denominator}, reduced to lowest terms.
   *
   * @param numerator the numerator
   * @param denominator the denominator, nonzero
   * @return the rational
   * @throws ArithmeticException if the denominator is zero
   */
  public static Rational of(long numerator, long denominator) {
    return of(BigInteger.valueOf(numerator), BigInteger.valueOf(denominator));
  }

  /**
   * The numerator in lowest terms.
   *
   * @return the numerator, signed
   */
  public BigInteger numerator() {
    return numerator;
  }

  /**
   * The denominator in lowest terms.
   *
   * @return the denominator, positive
   */
  public BigInteger denominator() {
    return denominator;
  }

  /**
   * Whether a {@code double} holds this rational exactly: it is a dyadic fraction within the
   * {@code double} range and precision.
   *
   * @return {@code true} when {@link #doubleValue()} loses nothing
   */
  public boolean isBinary64() {
    double nearest = doubleValue();
    return !Double.isInfinite(nearest) && of(new BigDecimal(nearest)).equals(this);
  }

  private static Rational of(BigDecimal exact) {
    BigInteger unscaled = exact.unscaledValue();
    int scale = exact.scale();
    return scale >= 0
        ? of(unscaled, BigInteger.TEN.pow(scale))
        : of(unscaled.multiply(BigInteger.TEN.pow(-scale)), BigInteger.ONE);
  }

  /**
   * The nearest {@code double}, ties to even; infinite only past the finite range.
   *
   * @return the nearest double
   */
  @Override
  public double doubleValue() {
    if (numerator.signum() == 0) {
      return 0.0;
    }
    BigInteger a = numerator.abs();
    // 62 or 63 significant bits of the quotient, and a sticky bit for any remainder, round once.
    int shift = 62 - (a.bitLength() - denominator.bitLength());
    BigInteger[] qr =
        shift >= 0
            ? a.shiftLeft(shift).divideAndRemainder(denominator)
            : a.divideAndRemainder(denominator.shiftLeft(-shift));
    BigInteger sticky =
        qr[0].shiftLeft(1).add(qr[1].signum() == 0 ? BigInteger.ZERO : BigInteger.ONE);
    int exponent = shift + 1;
    BigDecimal dyadic =
        exponent >= 0
            ? new BigDecimal(sticky.multiply(BigInteger.valueOf(5).pow(exponent)), exponent)
            : new BigDecimal(sticky.shiftLeft(-exponent));
    double magnitude = Double.parseDouble(dyadic.toString());
    return numerator.signum() < 0 ? -magnitude : magnitude;
  }

  @Override
  public float floatValue() {
    return (float) doubleValue();
  }

  /** The integer part, truncated toward zero, as {@link BigInteger#intValue} narrows it. */
  @Override
  public int intValue() {
    return numerator.divide(denominator).intValue();
  }

  /** The integer part, truncated toward zero, as {@link BigInteger#longValue} narrows it. */
  @Override
  public long longValue() {
    return numerator.divide(denominator).longValue();
  }

  @Override
  public int compareTo(Rational other) {
    return numerator.multiply(other.denominator).compareTo(other.numerator.multiply(denominator));
  }

  @Override
  public boolean equals(Object other) {
    return other instanceof Rational r
        && numerator.equals(r.numerator)
        && denominator.equals(r.denominator);
  }

  @Override
  public int hashCode() {
    return Objects.hash(numerator, denominator);
  }

  /** {@code numerator/denominator}, as the wire and the evaluator's {@code ToRational} spell it. */
  @Override
  public String toString() {
    return numerator + "/" + denominator;
  }
}
