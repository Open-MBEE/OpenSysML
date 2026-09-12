package org.openmbee.opensysml.internal;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertNotEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

import org.openmbee.opensysml.Instance;
import org.openmbee.opensysml.Quantity;
import org.openmbee.opensysml.TransportException;
import org.openmbee.opensysml.Value;
import org.openmbee.opensysml.proto.Array;
import org.openmbee.opensysml.proto.AttributeInfo;
import org.openmbee.opensysml.proto.Complex;
import org.openmbee.opensysml.proto.FeatureValue;
import org.openmbee.opensysml.proto.Function;
import org.openmbee.opensysml.proto.MeasurementRef;
import org.openmbee.opensysml.proto.SymbolInfo;
import org.openmbee.opensysml.proto.TensorQuantity;
import org.openmbee.opensysml.proto.UnitFactor;
import org.openmbee.opensysml.proto.UnitTerm;
import org.openmbee.opensysml.proto.ValueSequence;
import org.openmbee.opensysml.proto.ValueSet;
import org.openmbee.opensysml.proto.Vector;
import org.openmbee.opensysml.proto.VectorQuantity;
import java.util.List;
import java.util.Optional;
import org.junit.jupiter.api.Test;

/** Reading the wire into the client's own types, including what a future service may send. */
class ProtosTest {

  @Test
  void aValueOfNoKindIsAbsentRatherThanGuessed() {
    assertEquals(Optional.empty(), Protos.value(org.openmbee.opensysml.proto.Value.getDefaultInstance()));
  }

  @Test
  void aComplexNumberReadsAsOneValueWithBothParts() {
    org.openmbee.opensysml.proto.Value complex =
        org.openmbee.opensysml.proto.Value.newBuilder()
            .setComplex(Complex.newBuilder().setReal(1.5).setImaginary(-2.0))
            .build();
    assertEquals(Optional.of(new Value.ComplexValue(1.5, -2.0)), Protos.value(complex));

    // Proto3 defaults are zero, so a Complex with neither part set is 0 + 0i.
    org.openmbee.opensysml.proto.Value zero =
        org.openmbee.opensysml.proto.Value.newBuilder()
            .setComplex(Complex.getDefaultInstance())
            .build();
    assertEquals(Optional.of(new Value.ComplexValue(0.0, 0.0)), Protos.value(zero));

    org.openmbee.opensysml.proto.Value sequence =
        org.openmbee.opensysml.proto.Value.newBuilder()
            .setSequence(
                ValueSequence.newBuilder()
                    .addElements(complex)
                    .addElements(
                        org.openmbee.opensysml.proto.Value.newBuilder()
                            .setComplex(Complex.newBuilder().setReal(3.0).setImaginary(4.0))))
            .build();
    assertEquals(
        Optional.of(
            new Value.Sequence(
                List.of(new Value.ComplexValue(1.5, -2.0), new Value.ComplexValue(3.0, 4.0)))),
        Protos.value(sequence));
  }

  private static org.openmbee.opensysml.proto.Value integer(long value) {
    return org.openmbee.opensysml.proto.Value.newBuilder().setIntValue(value).build();
  }

  private static org.openmbee.opensysml.proto.Value real(double value) {
    return org.openmbee.opensysml.proto.Value.newBuilder().setRealValue(value).build();
  }

  private static org.openmbee.opensysml.proto.Quantity metres(double magnitude) {
    return org.openmbee.opensysml.proto.Quantity.newBuilder()
        .setRealMagnitude(magnitude)
        .setUnit("m")
        .setUnitTerm(
            UnitTerm.newBuilder()
                .setScaleNum(1.0)
                .setScaleDen(1.0)
                .addFactors(UnitFactor.newBuilder().setUnitId("SI::metre").setExponent(1.0)))
        .build();
  }

  private static final Quantity.UnitTerm METRE =
      new Quantity.UnitTerm(1.0, 1.0, List.of(new Quantity.UnitFactor("SI::metre", 1.0)));

  private static org.openmbee.opensysml.proto.Value array(
      List<Long> dimensions, org.openmbee.opensysml.proto.Value... elements) {
    return org.openmbee.opensysml.proto.Value.newBuilder()
        .setArray(Array.newBuilder().addAllDimensions(dimensions).addAllElements(List.of(elements)))
        .build();
  }

  private static org.openmbee.opensysml.proto.Value vector(
      org.openmbee.opensysml.proto.Value... components) {
    return org.openmbee.opensysml.proto.Value.newBuilder()
        .setVector(Vector.newBuilder().addAllComponents(List.of(components)))
        .build();
  }

  private static org.openmbee.opensysml.proto.Value vectorQuantity(
      org.openmbee.opensysml.proto.Quantity... components) {
    return org.openmbee.opensysml.proto.Value.newBuilder()
        .setVectorQuantity(VectorQuantity.newBuilder().addAllComponents(List.of(components)))
        .build();
  }

  @Test
  void anArrayReadsWithItsShapeAndItsElementsInRowMajorOrder() {
    Value.ArrayValue grid =
        (Value.ArrayValue)
            Protos.value(
                    array(
                        List.of(2L, 3L),
                        integer(1), integer(2), integer(3), integer(4), integer(5), integer(6)))
                .orElseThrow();
    assertEquals(List.of(2L, 3L), grid.dimensions());
    assertEquals(2, grid.rank());
    assertEquals(new Value.IntegerValue(6), grid.get(1, 2));
    assertEquals(new Value.IntegerValue(2), grid.get(0, 1));
    assertEquals(
        new Value.ArrayValue(
            List.of(2L, 3L),
            List.of(
                new Value.IntegerValue(1), new Value.IntegerValue(2), new Value.IntegerValue(3),
                new Value.IntegerValue(4), new Value.IntegerValue(5), new Value.IntegerValue(6))),
        grid);

    // Rank 0 holds exactly one element; rank 1 and 3 keep every extent.
    Value.ArrayValue scalar =
        (Value.ArrayValue) Protos.value(array(List.of(), real(7.0))).orElseThrow();
    assertEquals(0, scalar.rank());
    assertEquals(new Value.RealValue(7.0), scalar.get());
    Value.ArrayValue cube =
        (Value.ArrayValue)
            Protos.value(
                    array(
                        List.of(2L, 2L, 2L),
                        integer(0), integer(1), integer(2), integer(3),
                        integer(4), integer(5), integer(6), integer(7)))
                .orElseThrow();
    assertEquals(new Value.IntegerValue(5), cube.get(1, 0, 1));

    // An element is any value: a nested array holding a quantity, or a vector.
    Value nested =
        Protos.value(
                array(
                    List.of(2L),
                    array(
                        List.of(1L),
                        org.openmbee.opensysml.proto.Value.newBuilder().setQuantity(metres(3.0)).build()),
                    vector(real(1.0), real(2.0))))
            .orElseThrow();
    assertEquals(
        new Value.ArrayValue(
            List.of(2L),
            List.of(
                new Value.ArrayValue(
                    List.of(1L),
                    List.of(
                        new Value.QuantityValue(
                            new Quantity(3.0, Optional.of("m"), Optional.of(METRE))))),
                new Value.VectorValue(List.of(new Value.RealValue(1.0), new Value.RealValue(2.0))))),
        nested);
  }

  @Test
  void anArrayWhoseElementsDoNotFillItsShapeIsRefused() {
    org.openmbee.opensysml.proto.Value twoOfSix = array(List.of(2L, 3L), integer(1), integer(2));
    TransportException short_ =
        assertThrows(TransportException.class, () -> Protos.value(twoOfSix));
    assertTrue(short_.getMessage().contains("malformed array"), short_.getMessage());
    org.openmbee.opensysml.proto.Value zeroExtent = array(List.of(0L));
    assertThrows(TransportException.class, () -> Protos.value(zeroExtent));
    org.openmbee.opensysml.proto.Value negativeExtent = array(List.of(-1L), integer(1));
    assertThrows(TransportException.class, () -> Protos.value(negativeExtent));
    List<Long> shape = List.of(2L);
    List<Value> one = List.of(new Value.IntegerValue(1));
    assertThrows(IllegalArgumentException.class, () -> new Value.ArrayValue(shape, one));
    Value.ArrayValue two =
        new Value.ArrayValue(shape, List.of(new Value.IntegerValue(1), new Value.IntegerValue(2)));
    assertThrows(IndexOutOfBoundsException.class, () -> two.get(2));
    assertThrows(IndexOutOfBoundsException.class, () -> two.get(0, 0));
  }

  @Test
  void aVectorReadsAsOneValueKeepingIntegerAndRealApart() {
    Value.VectorValue reals =
        (Value.VectorValue) Protos.value(vector(real(3.0), real(4.0))).orElseThrow();
    assertEquals(2, reals.dimension());
    assertEquals(List.of(new Value.RealValue(3.0), new Value.RealValue(4.0)), reals.components());
    assertNotEquals(
        new Value.Sequence(List.of(new Value.RealValue(3.0), new Value.RealValue(4.0))), (Value) reals);
    assertEquals(
        new Value.VectorValue(List.of(new Value.IntegerValue(1), new Value.RealValue(2.5))),
        Protos.value(vector(integer(1), real(2.5))).orElseThrow());
    assertEquals(new Value.VectorValue(List.of()), Protos.value(vector()).orElseThrow());

    org.openmbee.opensysml.proto.Value oneThenTwo =
        vector(real(1.0), org.openmbee.opensysml.proto.Value.newBuilder().setStringValue("two").build());
    TransportException text =
        assertThrows(TransportException.class, () -> Protos.value(oneThenTwo));
    assertTrue(text.getMessage().contains("not a number"), text.getMessage());
    org.openmbee.opensysml.proto.Value unset = vector(org.openmbee.opensysml.proto.Value.getDefaultInstance());
    assertThrows(TransportException.class, () -> Protos.value(unset));
    List<Value> aBoolean = List.of(new Value.BooleanValue(true));
    assertThrows(IllegalArgumentException.class, () -> new Value.VectorValue(aBoolean));
  }

  @Test
  void aVectorQuantityReadsOneQuantityPerComponentEachWithItsUnit() {
    Value.VectorQuantityValue position =
        (Value.VectorQuantityValue)
            Protos.value(vectorQuantity(metres(3.0), metres(4.0))).orElseThrow();
    assertEquals(2, position.dimension());
    assertEquals(Optional.of("m"), position.unit());
    assertEquals(
        List.of(
            new Quantity(3.0, Optional.of("m"), Optional.of(METRE)),
            new Quantity(4.0, Optional.of("m"), Optional.of(METRE))),
        position.components());

    // The units may differ per component, and a composed unit keeps its reduction.
    org.openmbee.opensysml.proto.Quantity speed =
        org.openmbee.opensysml.proto.Quantity.newBuilder()
            .setRealMagnitude(5.0)
            .setUnit("m/s")
            .setUnitTerm(
                UnitTerm.newBuilder()
                    .setScaleNum(1.0)
                    .setScaleDen(1.0)
                    .addFactors(UnitFactor.newBuilder().setUnitId("SI::metre").setExponent(1.0))
                    .addFactors(UnitFactor.newBuilder().setUnitId("SI::second").setExponent(-1.0)))
            .build();
    Value.VectorQuantityValue mixed =
        (Value.VectorQuantityValue) Protos.value(vectorQuantity(metres(1.0), speed)).orElseThrow();
    assertEquals(Optional.empty(), mixed.unit());
    assertEquals(Optional.of("m/s"), mixed.components().get(1).unit());
    assertEquals(
        List.of(
            new Quantity.UnitFactor("SI::metre", 1.0), new Quantity.UnitFactor("SI::second", -1.0)),
        mixed.components().get(1).reduction().orElseThrow().factors());

    org.openmbee.opensysml.proto.Value noComponents = vectorQuantity();
    TransportException empty =
        assertThrows(TransportException.class, () -> Protos.value(noComponents));
    assertTrue(empty.getMessage().contains("no components"), empty.getMessage());
    List<Quantity> none = List.of();
    assertThrows(IllegalArgumentException.class, () -> new Value.VectorQuantityValue(none));
  }

  private static org.openmbee.opensysml.proto.Value set(
      org.openmbee.opensysml.proto.Value... elements) {
    return org.openmbee.opensysml.proto.Value.newBuilder()
        .setSet(ValueSet.newBuilder().addAllElements(List.of(elements)))
        .build();
  }

  private static org.openmbee.opensysml.proto.Value tensor(
      List<Long> dimensions, org.openmbee.opensysml.proto.Quantity... components) {
    return org.openmbee.opensysml.proto.Value.newBuilder()
        .setTensorQuantity(
            TensorQuantity.newBuilder()
                .addAllDimensions(dimensions)
                .addAllComponents(List.of(components)))
        .build();
  }

  @Test
  void aSetHoldsEachMemberOnceAndComparesInAnyOrder() {
    Value.SetValue members =
        (Value.SetValue) Protos.value(set(integer(1), integer(2), integer(3))).orElseThrow();
    assertEquals(3, members.size());
    assertTrue(!members.isEmpty());
    assertEquals(
        List.of(new Value.IntegerValue(1), new Value.IntegerValue(2), new Value.IntegerValue(3)),
        members.elements());
    assertTrue(members.contains(new Value.IntegerValue(2)));
    assertTrue(!members.contains(new Value.IntegerValue(4)));
    assertTrue(members.contains(new Value.RealValue(2.0)));
    assertTrue(!members.contains(new Value.RealValue(2.5)));

    // The same members in another order are the same set, with the same hash; a sequence is not.
    Value.SetValue reordered =
        new Value.SetValue(
            List.of(new Value.IntegerValue(3), new Value.IntegerValue(1), new Value.IntegerValue(2)));
    assertEquals(members, reordered);
    assertEquals(members.hashCode(), reordered.hashCode());
    Value inOrder =
        new Value.Sequence(
            List.of(new Value.IntegerValue(1), new Value.IntegerValue(2), new Value.IntegerValue(3)));
    assertFalse(members.sameValue(inOrder));
    assertFalse(inOrder.sameValue(members));
    assertNotEquals(
        members, new Value.SetValue(List.of(new Value.IntegerValue(1), new Value.IntegerValue(2))));

    // An empty set is a set; a set nests in a set and in a sequence.
    Value.SetValue empty = (Value.SetValue) Protos.value(set()).orElseThrow();
    assertTrue(empty.isEmpty());
    Value.SetValue nested = (Value.SetValue) Protos.value(set(set(integer(1)), set())).orElseThrow();
    assertEquals(2, nested.size());
    assertTrue(nested.contains(empty));
    Value.Sequence holding =
        (Value.Sequence)
            Protos.value(
                    org.openmbee.opensysml.proto.Value.newBuilder()
                        .setSequence(
                            ValueSequence.newBuilder().addElements(set(integer(1))).addElements(integer(2)))
                        .build())
                .orElseThrow();
    assertEquals(new Value.SetValue(List.of(new Value.IntegerValue(1))), holding.elements().get(0));

    // A member listed twice is not a set, on the wire or in the constructor.
    org.openmbee.opensysml.proto.Value twice = set(integer(1), integer(1));
    TransportException repeated =
        assertThrows(TransportException.class, () -> Protos.value(twice));
    assertTrue(repeated.getMessage().contains("twice"), repeated.getMessage());
    List<Value> duplicated = List.of(new Value.StringValue("a"), new Value.StringValue("a"));
    assertThrows(IllegalArgumentException.class, () -> new Value.SetValue(duplicated));
    org.openmbee.opensysml.proto.Value unknown =
        set(org.openmbee.opensysml.proto.Value.getDefaultInstance());
    assertThrows(TransportException.class, () -> Protos.value(unknown));

    // A null and the empty collections are one member: the absent value however spelt.
    org.openmbee.opensysml.proto.Value nul =
        org.openmbee.opensysml.proto.Value.newBuilder().setNull("").build();
    org.openmbee.opensysml.proto.Value emptySequence =
        org.openmbee.opensysml.proto.Value.newBuilder()
            .setSequence(ValueSequence.newBuilder())
            .build();
    for (org.openmbee.opensysml.proto.Value spelt :
        List.of(
            set(nul, emptySequence),
            set(integer(1), nul, set()),
            set(emptySequence, set()),
            set(set(nul, integer(1)), set(integer(1), emptySequence)))) {
      TransportException absent = assertThrows(TransportException.class, () -> Protos.value(spelt));
      assertTrue(absent.getMessage().contains("twice"), absent.getMessage());
    }
    assertEquals(2, ((Value.SetValue) Protos.value(set(nul, integer(0))).orElseThrow()).size());
    assertEquals(
        2, ((Value.SetValue) Protos.value(set(set(), set(set()))).orElseThrow()).size());
  }

  @Test
  void aTensorQuantityKeepsItsRankShapeAndRowMajorComponents() {
    Value.TensorQuantityValue cube =
        (Value.TensorQuantityValue)
            Protos.value(
                    tensor(
                        List.of(2L, 2L, 2L),
                        metres(1), metres(2), metres(3), metres(4),
                        metres(5), metres(6), metres(7), metres(8)))
                .orElseThrow();
    assertEquals(3, cube.rank());
    assertEquals(List.of(2L, 2L, 2L), cube.dimensions());
    assertEquals(8, cube.components().size());
    assertEquals(Optional.of("m"), cube.unit());
    assertEquals(6.0, cube.get(1, 0, 1).magnitude().doubleValue());
    assertEquals(1.0, cube.get(0, 0, 0).magnitude().doubleValue());
    assertEquals(8.0, cube.get(1, 1, 1).magnitude().doubleValue());
    assertThrows(IndexOutOfBoundsException.class, () -> cube.get(1, 1));
    assertThrows(IndexOutOfBoundsException.class, () -> cube.get(1, 1, 1, 0));
    assertThrows(IndexOutOfBoundsException.class, () -> cube.get(2, 0, 0));
    assertThrows(IndexOutOfBoundsException.class, () -> cube.get(0, -1, 0));

    // A rank-one tensor stays a tensor, never a vector quantity.
    Value line = Protos.value(tensor(List.of(2L), metres(1), metres(2))).orElseThrow();
    assertTrue(line instanceof Value.TensorQuantityValue, line.toString());
    assertNotEquals(
        line,
        new Value.VectorQuantityValue(
            List.of(
                new Quantity(1.0, Optional.of("m"), Optional.of(METRE)),
                new Quantity(2.0, Optional.of("m"), Optional.of(METRE)))));

    // Components with differing units report no shared one.
    org.openmbee.opensysml.proto.Quantity speed =
        org.openmbee.opensysml.proto.Quantity.newBuilder().setIntMagnitude(5).setUnit("m/s").build();
    Value.TensorQuantityValue mixed =
        (Value.TensorQuantityValue)
            Protos.value(tensor(List.of(1L, 2L), metres(1), speed)).orElseThrow();
    assertEquals(Optional.empty(), mixed.unit());
    assertEquals(5L, mixed.get(0, 1).magnitude());

    // Shape and components must agree, and every dimension is positive.
    org.openmbee.opensysml.proto.Value shortOne = tensor(List.of(2L, 2L), metres(1), metres(2), metres(3));
    TransportException few = assertThrows(TransportException.class, () -> Protos.value(shortOne));
    assertTrue(few.getMessage().contains("want 4"), few.getMessage());
    org.openmbee.opensysml.proto.Value scalarWithTwo = tensor(List.of(), metres(1), metres(2));
    assertThrows(TransportException.class, () -> Protos.value(scalarWithTwo));
    org.openmbee.opensysml.proto.Value zero = tensor(List.of(0L));
    TransportException nonPositive = assertThrows(TransportException.class, () -> Protos.value(zero));
    assertTrue(nonPositive.getMessage().contains("not positive"), nonPositive.getMessage());
    org.openmbee.opensysml.proto.Value negative = tensor(List.of(-1L), metres(1));
    assertThrows(TransportException.class, () -> Protos.value(negative));
    org.openmbee.opensysml.proto.Value overflow = tensor(List.of(Long.MAX_VALUE, 2L));
    assertThrows(TransportException.class, () -> Protos.value(overflow));
    org.openmbee.opensysml.proto.Value noMagnitude =
        tensor(List.of(1L), org.openmbee.opensysml.proto.Quantity.newBuilder().setUnit("m").build());
    TransportException unmeasured =
        assertThrows(TransportException.class, () -> Protos.value(noMagnitude));
    assertTrue(unmeasured.getMessage().contains("no magnitude"), unmeasured.getMessage());
    List<Long> twoWide = List.of(2L);
    List<Quantity> none = List.of();
    assertThrows(IllegalArgumentException.class, () -> new Value.TensorQuantityValue(twoWide, none));
  }

  private static org.openmbee.opensysml.proto.Value measurementRef(MeasurementRef.Builder ref) {
    return org.openmbee.opensysml.proto.Value.newBuilder().setMeasurementRef(ref).build();
  }

  private static UnitTerm.Builder unitTerm(double scale, UnitFactor.Builder... factors) {
    UnitTerm.Builder term = UnitTerm.newBuilder().setScaleNum(scale).setScaleDen(1.0);
    for (UnitFactor.Builder factor : factors) {
      term.addFactors(factor);
    }
    return term;
  }

  private static UnitFactor.Builder factor(String unitId, double exponent) {
    return UnitFactor.newBuilder().setUnitId(unitId).setExponent(exponent);
  }

  @Test
  void aMeasurementReferenceReadsItsUnitItsReductionAndTheDeclarationItNames() {
    Value.MeasurementRefValue km =
        (Value.MeasurementRefValue)
            Protos.value(
                    measurementRef(
                        MeasurementRef.newBuilder()
                            .setUnit("km")
                            .setUnitId("SI::kilometre")
                            .setUnitTerm(unitTerm(1000.0, factor("SI::metre", 1.0)))))
                .orElseThrow();
    assertEquals("km", km.unit());
    assertEquals(Optional.of("SI::kilometre"), km.unitId());
    assertEquals(
        new Quantity.UnitTerm(1000.0, 1.0, List.of(new Quantity.UnitFactor("SI::metre", 1.0))),
        km.reduction());

    // A unit an operation composed names no declaration, and none is invented for it.
    Value.MeasurementRefValue speed =
        (Value.MeasurementRefValue)
            Protos.value(
                    measurementRef(
                        MeasurementRef.newBuilder()
                            .setUnit("m/s")
                            .setUnitTerm(
                                unitTerm(1.0, factor("SI::metre", 1.0), factor("SI::second", -1.0)))))
                .orElseThrow();
    assertEquals(Optional.empty(), speed.unitId());
    assertEquals(
        List.of(
            new Quantity.UnitFactor("SI::metre", 1.0), new Quantity.UnitFactor("SI::second", -1.0)),
        speed.reduction().factors());

    // Naming no unit, or a unit without its reduction, is malformed at any depth.
    org.openmbee.opensysml.proto.Value noUnit = measurementRef(MeasurementRef.newBuilder());
    TransportException nothing =
        assertThrows(TransportException.class, () -> Protos.value(noUnit));
    assertTrue(nothing.getMessage().contains("names no unit"), nothing.getMessage());
    org.openmbee.opensysml.proto.Value noReduction =
        measurementRef(MeasurementRef.newBuilder().setUnit("km"));
    TransportException unreduced =
        assertThrows(TransportException.class, () -> Protos.value(noReduction));
    assertTrue(unreduced.getMessage().contains("km: it has no reduction"), unreduced.getMessage());
    org.openmbee.opensysml.proto.Value nested =
        org.openmbee.opensysml.proto.Value.newBuilder()
            .setSequence(
                ValueSequence.newBuilder()
                    .addElements(measurementRef(MeasurementRef.newBuilder().setUnitId("SI::kilometre"))))
            .build();
    assertThrows(TransportException.class, () -> Protos.value(nested));
  }

  private static org.openmbee.opensysml.proto.Value function(String calcId, long selfId) {
    return org.openmbee.opensysml.proto.Value.newBuilder()
        .setFunction(Function.newBuilder().setCalcId(calcId).setSelfId(selfId))
        .build();
  }

  @Test
  void aFunctionIsTheCalcItNamesReadAgainstAnObjectOrNone() {
    assertEquals(
        Optional.of(new Value.FunctionValue("Demo::Sq", Optional.empty())),
        Protos.value(function("Demo::Sq", 0)));
    assertEquals(
        Optional.of(new Value.FunctionValue("Demo::Scaler::scale", Optional.of(7L))),
        Protos.value(function("Demo::Scaler::scale", 7)));
    // The object is part of the identity: another object's read is another value.
    assertNotEquals(
        Protos.value(function("Demo::Scaler::scale", 7)),
        Protos.value(function("Demo::Scaler::scale", 8)));

    // Naming no calc is malformed at any depth, on the wire and in the record.
    org.openmbee.opensysml.proto.Value noCalc = function("", 0);
    TransportException nothing =
        assertThrows(TransportException.class, () -> Protos.value(noCalc));
    assertTrue(nothing.getMessage().contains("names no calc"), nothing.getMessage());
    org.openmbee.opensysml.proto.Value nested =
        org.openmbee.opensysml.proto.Value.newBuilder()
            .setSequence(ValueSequence.newBuilder().addElements(function("", 3)))
            .build();
    assertThrows(TransportException.class, () -> Protos.value(nested));
    assertThrows(
        IllegalArgumentException.class, () -> new Value.FunctionValue("", Optional.empty()));
  }

  private static org.openmbee.opensysml.proto.Value metaobject(
      String elementId, String metaclassId) {
    return org.openmbee.opensysml.proto.Value.newBuilder()
        .setMetaobject(
            org.openmbee.opensysml.proto.Metaobject.newBuilder()
                .setElementId(elementId)
                .setMetaclassId(metaclassId))
        .build();
  }

  @Test
  void aMetaobjectIsTheElementItReflectsOnUnderItsOwnMetaclass() {
    Value seatBelt =
        Protos.value(metaobject("Demo::seatBelt", "SysML::Systems::PartUsage")).orElseThrow();
    assertEquals(new Value.MetaobjectValue("Demo::seatBelt", "SysML::Systems::PartUsage"), seatBelt);
    assertEquals(
        "SysML::Systems::PartUsage", ((Value.MetaobjectValue) seatBelt).metaclassId());

    // The element is the identity: the type it was cast to does not distinguish two reads.
    Value asFeature = Protos.value(metaobject("Demo::seatBelt", "KerML::Feature")).orElseThrow();
    assertEquals(seatBelt, asFeature);
    assertEquals(seatBelt.hashCode(), asFeature.hashCode());
    assertTrue(seatBelt.sameValue(asFeature));
    assertNotEquals(
        seatBelt, Protos.value(metaobject("Demo::Vehicle", "SysML::Systems::PartUsage")).orElseThrow());
    assertFalse(seatBelt.sameValue(new Value.StringValue("Demo::seatBelt")));
    org.openmbee.opensysml.proto.Value twice =
        org.openmbee.opensysml.proto.Value.newBuilder()
            .setSet(
                ValueSet.newBuilder()
                    .addElements(metaobject("Demo::seatBelt", "KerML::Feature"))
                    .addElements(metaobject("Demo::seatBelt", "KerML::Type")))
            .build();
    assertThrows(TransportException.class, () -> Protos.value(twice));

    // Naming no element is malformed at any depth, on the wire and in the record.
    org.openmbee.opensysml.proto.Value noElement = metaobject("", "KerML::Feature");
    TransportException nothing =
        assertThrows(TransportException.class, () -> Protos.value(noElement));
    assertTrue(nothing.getMessage().contains("names no element"), nothing.getMessage());
    org.openmbee.opensysml.proto.Value nested =
        org.openmbee.opensysml.proto.Value.newBuilder()
            .setSequence(ValueSequence.newBuilder().addElements(metaobject("", "")))
            .build();
    assertThrows(TransportException.class, () -> Protos.value(nested));
    assertThrows(IllegalArgumentException.class, () -> new Value.MetaobjectValue("", ""));
  }

  @Test
  void aQuantityWithoutAMagnitudeIsRefusedRatherThanReadAsZero() {
    org.openmbee.opensysml.proto.Quantity noMagnitude =
        org.openmbee.opensysml.proto.Quantity.newBuilder().setUnit("m").build();

    org.openmbee.opensysml.proto.Value partlyUnmeasured = vectorQuantity(metres(3.0), noMagnitude);
    TransportException component =
        assertThrows(TransportException.class, () -> Protos.value(partlyUnmeasured));
    assertTrue(component.getMessage().contains("no magnitude"), component.getMessage());

    org.openmbee.opensysml.proto.Value alone =
        org.openmbee.opensysml.proto.Value.newBuilder().setQuantity(noMagnitude).build();
    assertThrows(TransportException.class, () -> Protos.value(alone));
  }

  @Test
  void aStructuredValueSurvivesTheWireBytes() throws Exception {
    for (org.openmbee.opensysml.proto.Value value :
        List.of(
            array(List.of(2L, 3L), integer(1), integer(2), integer(3), integer(4), integer(5), integer(6)),
            vector(real(3.0), integer(4)),
            vectorQuantity(metres(3.0), metres(4.0)),
            measurementRef(
                MeasurementRef.newBuilder()
                    .setUnit("km")
                    .setUnitId("SI::kilometre")
                    .setUnitTerm(unitTerm(1000.0, factor("SI::metre", 1.0)))))) {
      org.openmbee.opensysml.proto.Value again =
          org.openmbee.opensysml.proto.Value.parseFrom(value.toByteArray());
      assertEquals(Protos.value(value), Protos.value(again));
    }
  }

  @Test
  void aSequenceElementOfAKindTheClientCannotReadIsRefusedRatherThanDropped() {
    // A newer service sending a value kind this release has no arm for must not silently shorten
    // the sequence it belongs to.
    org.openmbee.opensysml.proto.Value sequence =
        org.openmbee.opensysml.proto.Value.newBuilder()
            .setSequence(
                ValueSequence.newBuilder()
                    .addElements(org.openmbee.opensysml.proto.Value.newBuilder().setIntValue(1))
                    .addElements(org.openmbee.opensysml.proto.Value.getDefaultInstance())
                    .addElements(org.openmbee.opensysml.proto.Value.newBuilder().setIntValue(3)))
            .build();
    TransportException failed = assertThrows(TransportException.class, () -> Protos.value(sequence));
    assertTrue(failed.getMessage().contains("does not know"), failed.getMessage());
  }

  @Test
  void aFeatureValueOfAKindTheClientCannotReadIsRefusedRatherThanDropped() {
    org.openmbee.opensysml.proto.Instance instance =
        org.openmbee.opensysml.proto.Instance.newBuilder()
            .setId(1)
            .setTypeSymbolId("Demo::Vehicle")
            .putFeatureValues(
                "wheels",
                FeatureValue.newBuilder()
                    .setFeatureName("wheels")
                    .addValues(org.openmbee.opensysml.proto.Value.newBuilder().setInstanceId(2))
                    .addValues(org.openmbee.opensysml.proto.Value.getDefaultInstance())
                    .setMaterialized(true)
                    .build())
            .build();
    assertThrows(TransportException.class, () -> Protos.instance(instance));
  }

  @Test
  void aSingularValueOfAKindTheClientCannotReadIsRefusedRatherThanReadAsNoValue() {
    org.openmbee.opensysml.proto.Instance instance =
        org.openmbee.opensysml.proto.Instance.newBuilder()
            .setId(1)
            .setTypeSymbolId("Demo::Vehicle")
            .putFeatureValues(
                "mass",
                FeatureValue.newBuilder()
                    .setFeatureName("mass")
                    .setValue(org.openmbee.opensysml.proto.Value.getDefaultInstance())
                    .setMaterialized(true)
                    .build())
            .build();
    assertThrows(TransportException.class, () -> Protos.instance(instance));

    SymbolInfo symbol =
        SymbolInfo.newBuilder()
            .setId("Demo::Vehicle")
            .setName("Vehicle")
            .setKind("partDef")
            .addAttributes(
                AttributeInfo.newBuilder()
                    .setName("mass")
                    .setValue(org.openmbee.opensysml.proto.Value.getDefaultInstance()))
            .build();
    assertThrows(TransportException.class, () -> Protos.symbol(symbol));
  }

  @Test
  void anAttributeThatWasSentNoValueHoldsNone() {
    SymbolInfo symbol =
        SymbolInfo.newBuilder()
            .setId("Demo::Vehicle")
            .setName("Vehicle")
            .setKind("partDef")
            .addAttributes(AttributeInfo.newBuilder().setName("mass").setType("Real"))
            .build();
    assertEquals(Optional.empty(), Protos.symbol(symbol).attributes().get(0).value());
  }

  @Test
  void aSequenceOfReadableElementsKeepsItsOrderAndLength() {
    org.openmbee.opensysml.proto.Value sequence =
        org.openmbee.opensysml.proto.Value.newBuilder()
            .setSequence(
                ValueSequence.newBuilder()
                    .addElements(org.openmbee.opensysml.proto.Value.newBuilder().setIntValue(1))
                    .addElements(org.openmbee.opensysml.proto.Value.newBuilder().setUnset(true))
                    .addElements(org.openmbee.opensysml.proto.Value.newBuilder().setStringValue("x")))
            .build();
    assertEquals(
        Optional.of(
            new Value.Sequence(
                List.of(new Value.IntegerValue(1), new Value.UnsetValue(), new Value.StringValue("x")))),
        Protos.value(sequence));
  }

  @Test
  void anInstanceReadsItsFeatureValuesWholeAndImmutably() {
    org.openmbee.opensysml.proto.Instance instance =
        org.openmbee.opensysml.proto.Instance.newBuilder()
            .setId(7)
            .setTypeSymbolId("Demo::Vehicle")
            .putFeatureValues(
                "mass",
                FeatureValue.newBuilder()
                    .setFeatureName("mass")
                    .setValue(org.openmbee.opensysml.proto.Value.newBuilder().setRealValue(1200.0))
                    .setMaterialized(true)
                    .build())
            .build();
    Instance read = Protos.instance(instance);
    assertEquals(7, read.id());
    Instance.FeatureValue mass = read.featureValues().get("mass");
    assertEquals(Optional.of(new Value.RealValue(1200.0)), mass.value());
    assertEquals(List.of(), mass.values());
    assertTrue(mass.materialized());
    assertEquals(Optional.empty(), mass.error());
  }

  @Test
  void onlyAnAssertedInfinityArmIsTheUnboundedValue() {
    org.openmbee.opensysml.proto.Value asserted =
        org.openmbee.opensysml.proto.Value.newBuilder().setInfinity(true).build();
    assertEquals(Optional.of(new Value.InfinityValue()), Protos.value(asserted));

    org.openmbee.opensysml.proto.Value denied =
        org.openmbee.opensysml.proto.Value.newBuilder().setInfinity(false).build();
    assertThrows(TransportException.class, () -> Protos.value(denied));

    org.openmbee.opensysml.proto.Value nested =
        org.openmbee.opensysml.proto.Value.newBuilder()
            .setSequence(ValueSequence.newBuilder().addElements(denied))
            .build();
    assertThrows(TransportException.class, () -> Protos.value(nested));
  }
}
