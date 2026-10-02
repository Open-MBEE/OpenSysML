package org.openmbee.opensysml.internal;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

import java.math.BigInteger;
import java.util.List;
import org.openmbee.opensysml.DocumentValue;
import org.openmbee.opensysml.Quantity;
import org.openmbee.opensysml.Rational;
import org.openmbee.opensysml.TransportException;
import org.openmbee.opensysml.Value;
import org.openmbee.opensysml.proto.DocumentQueryBinding;
import org.openmbee.opensysml.proto.UnitFactor;
import org.openmbee.opensysml.proto.UnitTerm;
import org.openmbee.opensysml.proto.Vector;
import org.junit.jupiter.api.Test;

class RationalProtosTest {

  private static org.openmbee.opensysml.proto.Rational wire(String numerator, String denominator) {
    return org.openmbee.opensysml.proto.Rational.newBuilder()
        .setNumerator(numerator)
        .setDenominator(denominator)
        .build();
  }

  @Test
  void aRationalIsHeldInLowestTermsAndRoundedOnce() {
    Rational third = Rational.of(2, -6);
    assertEquals(BigInteger.valueOf(-1), third.numerator());
    assertEquals(BigInteger.valueOf(3), third.denominator());
    assertEquals("-1/3", third.toString());
    assertEquals(-1.0 / 3.0, third.doubleValue());
    assertEquals(0.1, Rational.of(1, 10).doubleValue());
    assertEquals(0.3, Rational.of(3, 10).doubleValue());
    assertFalse(Rational.of(1, 10).isBinary64());
    assertTrue(Rational.of(1, 4).isBinary64());
    assertTrue(Rational.of(6, 3).isBinary64());
    assertEquals(0, Rational.of(1, 3).compareTo(Rational.of(2, 6)));
    assertTrue(Rational.of(1, 3).compareTo(Rational.of(1, 2)) < 0);
    assertThrows(ArithmeticException.class, () -> Rational.of(1, 0));
    // A tie between two doubles rounds to even: 2^53 + 1 is halfway, 2^53 + 3 rounds up.
    assertEquals(0x1p53, Rational.of(BigInteger.ONE.shiftLeft(53).add(BigInteger.ONE),
        BigInteger.ONE).doubleValue());
    assertEquals(0x1p53 + 4, Rational.of(BigInteger.ONE.shiftLeft(53).add(BigInteger.valueOf(3)),
        BigInteger.ONE).doubleValue());
    assertEquals(Double.MIN_VALUE, Rational.of(BigInteger.ONE,
        BigInteger.ONE.shiftLeft(1074)).doubleValue());
    assertEquals(Double.POSITIVE_INFINITY, Rational.of(BigInteger.TEN.pow(400),
        BigInteger.valueOf(3)).doubleValue());
    assertEquals(0, Rational.of(1, 3).longValue());
    assertEquals(-2, Rational.of(-7, 3).intValue());
  }

  @Test
  void aRationalIsReadAndWrittenExactly() {
    var proto = org.openmbee.opensysml.proto.Value.newBuilder().setRationalValue(wire("1", "3")).build();
    Value read = Protos.value(proto).orElseThrow();
    assertEquals(new Value.RationalValue(Rational.of(1, 3)), read);
    assertEquals(proto, Protos.proto(read));
    assertEquals(1.0 / 3.0, read.asDouble());
    assertTrue(read.sameValue(new Value.RationalValue(Rational.of(2, 6))));
    assertFalse(read.sameValue(new Value.RealValue(1.0 / 3.0)));
    assertFalse(read.sameValue(new Value.IntegerValue(0)));
    var whole =
        org.openmbee.opensysml.proto.Value.newBuilder()
            .setRationalValue(wire(BigInteger.TEN.pow(400).toString(), "1"))
            .build();
    assertEquals(
        new Value.RationalValue(Rational.of(BigInteger.TEN.pow(400), BigInteger.ONE)),
        Protos.value(whole).orElseThrow());
    assertThrows(
        IllegalArgumentException.class, () -> new Value.RationalValue(Rational.of(1, 2)));
  }

  @Test
  void aRationalOnTheWireIsCanonical() {
    List<List<String>> malformed =
        List.of(
            List.of("2", "6"),
            List.of("1", "-3"),
            List.of("-1", "-3"),
            List.of("1", "0"),
            List.of("1", "2"),
            List.of("3", "1"),
            List.of("0", "1"),
            List.of("-0", "3"),
            List.of("01", "3"),
            List.of("+1", "3"),
            List.of("1.5", "7"),
            List.of("", "3"),
            List.of("1", ""));
    for (List<String> terms : malformed) {
      assertThrows(
          TransportException.class,
          () ->
              Protos.value(
                  org.openmbee.opensysml.proto.Value.newBuilder()
                      .setRationalValue(wire(terms.get(0), terms.get(1)))
                      .build()),
          terms.toString());
    }
  }

  @Test
  void aRationalMagnitudeAndComponentCrossExactly() {
    var wireQuantity =
        org.openmbee.opensysml.proto.Quantity.newBuilder()
            .setRationalMagnitude(wire("1", "3"))
            .setUnit("km")
            .setUnitTerm(
                UnitTerm.newBuilder()
                    .setScaleNum(1000)
                    .setScaleDen(1)
                    .addFactors(UnitFactor.newBuilder().setUnitId("SI::m").setExponent(1)))
            .build();
    Quantity third = Protos.quantity(wireQuantity);
    assertEquals(Rational.of(1, 3), third.magnitude());
    assertTrue(third.isExact());
    assertFalse(third.isIntegral());
    var metres =
        org.openmbee.opensysml.proto.Quantity.newBuilder()
            .setRationalMagnitude(wire("1000", "3"))
            .setUnit("m")
            .setUnitTerm(
                UnitTerm.newBuilder()
                    .setScaleNum(1)
                    .setScaleDen(1)
                    .addFactors(UnitFactor.newBuilder().setUnitId("SI::m").setExponent(1)))
            .build();
    assertTrue(
        new Value.QuantityValue(third).sameValue(new Value.QuantityValue(Protos.quantity(metres))));
    assertThrows(
        IllegalArgumentException.class,
        () -> new Quantity(Rational.of(1, 2), java.util.Optional.empty(), java.util.Optional.empty()));

    var vector =
        org.openmbee.opensysml.proto.Value.newBuilder()
            .setVector(
                Vector.newBuilder()
                    .addComponents(
                        org.openmbee.opensysml.proto.Value.newBuilder().setIntValue(1))
                    .addComponents(
                        org.openmbee.opensysml.proto.Value.newBuilder()
                            .setRationalValue(wire("2", "3"))))
            .build();
    assertEquals(
        new Value.VectorValue(
            List.of(new Value.IntegerValue(1), new Value.RationalValue(Rational.of(2, 3)))),
        Protos.value(vector).orElseThrow());
  }

  @Test
  void aDocumentBindingHoldingARationalNeedsTheCapability() {
    DocumentValue third = new DocumentValue.RationalValue(Rational.of(1, 3));
    var proto = Protos.proto(third);
    assertEquals(wire("1", "3"), proto.getRationalValue());
    assertEquals(third, Protos.documentValue(proto));
    assertTrue(
        Protos.holdsRational(DocumentQueryBinding.newBuilder().setParameter("x").addValues(proto).build()));
    assertFalse(
        Protos.holdsRational(
            DocumentQueryBinding.newBuilder()
                .setParameter("x")
                .addValues(Protos.proto(new DocumentValue.RealValue(0.5)))
                .build()));
  }
}
