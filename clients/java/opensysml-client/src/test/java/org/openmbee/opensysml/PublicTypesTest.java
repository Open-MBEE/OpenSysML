package org.openmbee.opensysml;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertInstanceOf;
import static org.junit.jupiter.api.Assertions.assertNotEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

import java.io.ByteArrayInputStream;
import java.io.ByteArrayOutputStream;
import java.io.IOException;
import java.io.InvalidObjectException;
import java.io.ObjectInputStream;
import java.io.ObjectOutputStream;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import org.junit.jupiter.api.Test;
import org.openmbee.opensysml.internal.Protos;

/** The public value types: immutable, comparable by value, and free of generated types. */
class PublicTypesTest {

  @Test
  void aSequenceCopiesWhatItWasGiven() {
    List<Value> elements = new ArrayList<>(List.of(new Value.IntegerValue(1)));
    Value.Sequence sequence = new Value.Sequence(elements);
    elements.add(new Value.IntegerValue(2));
    List<Value> copied = sequence.elements();
    Value added = new Value.NullValue();
    assertEquals(1, copied.size());
    assertThrows(UnsupportedOperationException.class, () -> copied.add(added));
  }

  @Test
  void valuesCompareByValue() {
    assertEquals(new Value.RealValue(1.5), new Value.RealValue(1.5));
    assertEquals(
        new Value.EnumerationValue(new EnumLiteral("D::Color::red", "D::Color", "Color::red")),
        new Value.EnumerationValue(new EnumLiteral("D::Color::red", "D::Color", "Color::red")));
    Value one = new Value.IntegerValue(1);
    Value oneAsReal = new Value.RealValue(1.0);
    assertNotEquals(one, oneAsReal);
  }

  @Test
  void aComplexValueIsOneNumberWithBothParts() {
    Value.ComplexValue z = new Value.ComplexValue(1.5, -2.0);
    assertEquals(new Value.ComplexValue(1.5, -2.0), z);
    assertEquals(new Value.ComplexValue(1.5, -2.0).hashCode(), z.hashCode());
    assertNotEquals(new Value.ComplexValue(1.5, 2.0), z);
    Value realPartOnly = new Value.RealValue(1.5);
    Value realThenImaginary =
        new Value.Sequence(List.of(new Value.RealValue(1.5), new Value.RealValue(-2.0)));
    Value zAsValue = z;
    assertNotEquals(realPartOnly, zAsValue);
    assertNotEquals(realThenImaginary, zAsValue);

    assertEquals("1.5 - 2.0i", z.format());
    assertEquals("1.0 + 2.0i", new Value.ComplexValue(1.0, 2.0).format());
    assertEquals("0.0 + 0.0i", new Value.ComplexValue(0.0, 0.0).format());
    assertEquals("-3.25 - 0.0i", new Value.ComplexValue(-3.25, -0.0).format());

    // A complex number has no single real magnitude, so it is not narrowed to one.
    assertThrows(IllegalStateException.class, z::asDouble);
    assertThrows(IllegalStateException.class, z::asLong);
  }

  @Test
  void aSetIsItsMembersInAnyOrderAndNoneTwice() {
    Value.SetValue set =
        new Value.SetValue(
            List.of(new Value.IntegerValue(3), new Value.IntegerValue(1), new Value.IntegerValue(2)));
    Value.SetValue reordered =
        new Value.SetValue(
            List.of(new Value.IntegerValue(1), new Value.IntegerValue(2), new Value.IntegerValue(3)));
    assertEquals(reordered, set);
    assertEquals(reordered.hashCode(), set.hashCode());
    assertEquals(3, set.size());
    assertFalse(set.isEmpty());
    assertTrue(set.contains(new Value.IntegerValue(2)));
    assertFalse(set.contains(new Value.IntegerValue(4)));
    assertEquals(List.of(3L, 1L, 2L), set.elements().stream().map(Value::asLong).toList());

    Value asSequence =
        new Value.Sequence(
            List.of(new Value.IntegerValue(1), new Value.IntegerValue(2), new Value.IntegerValue(3)));
    Value setAsValue = set;
    assertNotEquals(asSequence, setAsValue);
    assertNotEquals(new Value.SetValue(List.of(new Value.IntegerValue(1))), setAsValue);

    Value.SetValue empty = new Value.SetValue(List.of());
    assertTrue(empty.isEmpty());
    assertEquals(new Value.SetValue(List.of()), empty);
    assertEquals(
        new Value.SetValue(List.of(empty, set)), new Value.SetValue(List.of(set, empty)));

    List<Value> members = new ArrayList<>(List.of(new Value.IntegerValue(1)));
    Value.SetValue copied = new Value.SetValue(members);
    members.add(new Value.IntegerValue(2));
    assertEquals(1, copied.size());
    List<Value> exposed = copied.elements();
    Value added = new Value.IntegerValue(3);
    assertThrows(UnsupportedOperationException.class, () -> exposed.add(added));

    List<Value> twice = List.of(new Value.IntegerValue(1), new Value.IntegerValue(1));
    assertThrows(IllegalArgumentException.class, () -> new Value.SetValue(twice));
  }

  @Test
  void aSetJudgesItsMembersAsTheModelDoes() {
    Value one = new Value.IntegerValue(1);
    Value oneReal = new Value.RealValue(1.0);
    Value oneComplex = new Value.ComplexValue(1.0, 0.0);
    Value twoPointFive = new Value.RealValue(2.5);
    Value.SetValue set = new Value.SetValue(List.of(one, twoPointFive));

    // An Integer and the whole Real of its value are one member, as is a Complex on the real axis.
    assertTrue(set.contains(oneReal));
    assertTrue(set.contains(oneComplex));
    assertTrue(set.contains(new Value.ComplexValue(2.5, 0.0)));
    assertFalse(set.contains(new Value.RealValue(1.5)));
    assertFalse(set.contains(new Value.BooleanValue(true)));
    Value.SetValue byReals = new Value.SetValue(List.of(twoPointFive, oneReal));
    assertEquals(byReals, set);
    assertEquals(byReals.hashCode(), set.hashCode());
    assertNotEquals(one, oneReal);
    for (List<Value> twice :
        List.<List<Value>>of(
            List.of(one, oneReal),
            List.of(oneReal, oneComplex),
            List.of(one, oneComplex),
            List.of(new Value.IntegerValue(0), new Value.RealValue(-0.0)),
            List.of(new Value.QuantityValue(metres(1L)), new Value.QuantityValue(metres(1.0))))) {
      assertThrows(IllegalArgumentException.class, () -> new Value.SetValue(twice), twice::toString);
    }

    // Members that only look alike stay apart: nearby numbers beyond 2^53, a Complex off the
    // axis, a Boolean, another unit, another order of a sequence, another shape of an array.
    Value big = new Value.IntegerValue((1L << 53) + 1);
    Value bigReal = new Value.RealValue(0x1p53);
    for (List<Value> apart :
        List.<List<Value>>of(
            List.of(big, bigReal),
            List.of(new Value.IntegerValue(Long.MAX_VALUE), new Value.RealValue(0x1p63)),
            List.of(one, new Value.RealValue(1.5)),
            List.of(oneReal, new Value.ComplexValue(1.0, 1.0)),
            List.of(one, new Value.BooleanValue(true)),
            List.of(one, new Value.StringValue("1")),
            List.of(
                new Value.QuantityValue(metres(1L)),
                new Value.QuantityValue(new Quantity(1L, Optional.of("km"), Optional.empty()))),
            List.of(
                new Value.Sequence(List.of(one, twoPointFive)),
                new Value.Sequence(List.of(twoPointFive, one))),
            List.of(
                new Value.ArrayValue(List.of(2L, 1L), List.of(one, one)),
                new Value.ArrayValue(List.of(1L, 2L), List.of(one, one))),
            List.of(new Value.Sequence(List.of(one)), new Value.SetValue(List.of(one))))) {
      assertEquals(2, new Value.SetValue(apart).size(), apart::toString);
    }
    assertNotEquals(new Value.SetValue(List.of(big)), new Value.SetValue(List.of(bigReal)));
    assertEquals(
        new Value.SetValue(List.of(new Value.IntegerValue(Long.MIN_VALUE))),
        new Value.SetValue(List.of(new Value.RealValue(-0x1p63))));

    // Numbers nested in sequences, vectors, arrays and quantities are judged the same way.
    assertTrue(
        new Value.Sequence(List.of(one, twoPointFive))
            .sameValue(new Value.Sequence(List.of(oneReal, twoPointFive))));
    assertTrue(
        new Value.VectorValue(List.of(one, new Value.RealValue(2.0)))
            .sameValue(new Value.VectorValue(List.of(oneReal, new Value.IntegerValue(2)))));
    assertTrue(
        new Value.ArrayValue(List.of(1L), List.of(one))
            .sameValue(new Value.ArrayValue(List.of(1L), List.of(oneComplex))));
    assertTrue(
        new Value.VectorQuantityValue(List.of(metres(1L)))
            .sameValue(new Value.VectorQuantityValue(List.of(metres(1.0)))));
    assertTrue(
        new Value.TensorQuantityValue(List.of(1L, 1L), List.of(metres(1L)))
            .sameValue(new Value.TensorQuantityValue(List.of(1L, 1L), List.of(metres(1.0)))));
    assertFalse(
        new Value.TensorQuantityValue(List.of(1L, 1L), List.of(metres(1L)))
            .sameValue(new Value.TensorQuantityValue(List.of(1L), List.of(metres(1.0)))));
    assertFalse(new Value.NullValue().sameValue(new Value.UnsetValue()));
    assertTrue(new Value.NullValue().sameValue(new Value.NullValue()));
  }

  private static Quantity metres(Number magnitude) {
    return new Quantity(magnitude, Optional.of("m"), Optional.empty());
  }

  @Test
  void aTensorQuantityIsShapedAndIndexedInRowMajorOrder() {
    List<Quantity> pascals = new ArrayList<>();
    for (int i = 1; i <= 8; i++) {
      pascals.add(new Quantity((double) i, Optional.of("Pa"), Optional.empty()));
    }
    Value.TensorQuantityValue cube = new Value.TensorQuantityValue(List.of(2L, 2L, 2L), pascals);
    assertEquals(3, cube.rank());
    assertEquals(Optional.of("Pa"), cube.unit());
    assertEquals(1.0, cube.get(0, 0, 0).magnitude());
    assertEquals(6.0, cube.get(1, 0, 1).magnitude());
    assertEquals(8.0, cube.get(1, 1, 1).magnitude());
    assertEquals(new Value.TensorQuantityValue(List.of(2L, 2L, 2L), pascals), cube);
    assertNotEquals(new Value.TensorQuantityValue(List.of(2L, 4L), pascals), cube);

    assertThrows(IndexOutOfBoundsException.class, () -> cube.get(1, 1));
    assertThrows(IndexOutOfBoundsException.class, () -> cube.get(1, 1, 1, 1));
    assertThrows(IndexOutOfBoundsException.class, () -> cube.get(0, 2, 0));
    assertThrows(IndexOutOfBoundsException.class, () -> cube.get(0, -1, 0));

    Value.TensorQuantityValue line = new Value.TensorQuantityValue(List.of(2L), pascals.subList(0, 2));
    assertEquals(1, line.rank());
    Value lineAsValue = line;
    assertNotEquals(new Value.VectorQuantityValue(pascals.subList(0, 2)), lineAsValue);

    Value.TensorQuantityValue mixed =
        new Value.TensorQuantityValue(
            List.of(2L),
            List.of(
                new Quantity(1.0, Optional.of("m"), Optional.empty()),
                new Quantity(2.0, Optional.of("s"), Optional.empty())));
    assertEquals(Optional.empty(), mixed.unit());

    List<Quantity> seven = pascals.subList(0, 7);
    assertThrows(
        IllegalArgumentException.class, () -> new Value.TensorQuantityValue(List.of(2L, 2L, 2L), seven));
    assertThrows(
        IllegalArgumentException.class, () -> new Value.TensorQuantityValue(List.of(0L), List.of()));
    assertThrows(
        IllegalArgumentException.class,
        () -> new Value.TensorQuantityValue(List.of(-2L, -4L), pascals));
    IllegalArgumentException overflow =
        assertThrows(
            IllegalArgumentException.class,
            () -> new Value.TensorQuantityValue(List.of(Long.MAX_VALUE, 2L), pascals));
    assertInstanceOf(ArithmeticException.class, overflow.getCause());

    List<Long> shape = new ArrayList<>(List.of(8L));
    Value.TensorQuantityValue copied = new Value.TensorQuantityValue(shape, pascals);
    shape.set(0, 4L);
    assertEquals(List.of(8L), copied.dimensions());
  }

  @Test
  void anUnsetValueIsNotTheModelsNull() {
    Value unset = new Value.UnsetValue();
    Value modelsNull = new Value.NullValue();
    assertNotEquals(unset, modelsNull);
  }

  @Test
  void aQuantityKeepsIntegerAndRealApart() {
    Quantity integral = new Quantity(5L, Optional.of("kg"), Optional.empty());
    Quantity real = new Quantity(5.0, Optional.of("kg"), Optional.empty());
    assertTrue(integral.isIntegral());
    assertFalse(real.isIntegral());
    assertNotEquals(integral, real);
  }

  @Test
  void anInstanceCopiesItsFeatureValues() {
    Map<String, Instance.FeatureValue> featureValues = new java.util.LinkedHashMap<>();
    featureValues.put(
        "mass",
        new Instance.FeatureValue(
            "mass",
            Optional.of(new Value.RealValue(1500.0)),
            List.of(),
            true,
            Optional.empty()));
    Instance instance = new Instance(1, "Demo::Vehicle", featureValues);
    featureValues.clear();
    Map<String, Instance.FeatureValue> copied = instance.featureValues();
    assertEquals(1, copied.size());
    assertThrows(UnsupportedOperationException.class, copied::clear);
  }

  @Test
  void anInstantiationResolvesReferences() {
    Instance root = new Instance(1, "Demo::Vehicle", Map.of());
    Instance engine = new Instance(2, "Demo::Engine", Map.of());
    Instantiation instantiation = new Instantiation(root, List.of(root, engine), List.of());
    assertEquals(
        Optional.of(engine), instantiation.resolve(new Value.InstanceReference(2)));
    assertEquals(Optional.empty(), instantiation.resolve(new Value.InstanceReference(3)));
  }

  @Test
  void diagnosticSeverityReadsTheWireName() {
    assertEquals(Diagnostic.Severity.ERROR, Diagnostic.Severity.fromWireName("error"));
    assertEquals(Diagnostic.Severity.WARNING, Diagnostic.Severity.fromWireName("warning"));
    assertEquals(Diagnostic.Severity.INFO, Diagnostic.Severity.fromWireName("info"));
    assertEquals(Diagnostic.Severity.UNKNOWN, Diagnostic.Severity.fromWireName("hint"));
    assertEquals(Diagnostic.Severity.UNKNOWN, Diagnostic.Severity.fromWireName(""));
  }

  @Test
  void diagnosticCarriesTheWireCode() {
    List<Diagnostic> read =
        Protos.diagnostics(
            List.of(
                org.openmbee.opensysml.proto.Diagnostic.newBuilder()
                    .setSeverity("info")
                    .setMessage("choice point: 2 steppable tokens")
                    .setCode("choice-point")
                    .build(),
                org.openmbee.opensysml.proto.Diagnostic.newBuilder()
                    .setSeverity("error")
                    .setMessage("uncoded")
                    .build()));
    assertEquals("choice-point", read.get(0).code());
    assertEquals("", read.get(1).code());
    assertThrows(
        NullPointerException.class,
        () -> new Diagnostic(Diagnostic.Severity.ERROR, "m", null, Optional.empty()));
  }

  @Test
  void aModelExceptionPreservesDiagnosticsWhenSerialized() throws Exception {
    List<Diagnostic> diagnostics =
        List.of(
            new Diagnostic(
                Diagnostic.Severity.ERROR,
                "invalid model",
                "unresolved",
                Optional.of(new Diagnostic.Span("model.sysml", 2, 3, 2, 8))),
            new Diagnostic(Diagnostic.Severity.WARNING, "unlocated", "", Optional.empty()));
    ModelException original = new ModelException("rejected", diagnostics);

    ByteArrayOutputStream bytes = new ByteArrayOutputStream();
    try (ObjectOutputStream output = new ObjectOutputStream(bytes)) {
      output.writeObject(original);
    }
    try (ObjectInputStream input =
        new ObjectInputStream(new ByteArrayInputStream(bytes.toByteArray()))) {
      ModelException restored = (ModelException) input.readObject();
      assertEquals(original.getMessage(), restored.getMessage());
      List<Diagnostic> restoredDiagnostics = restored.diagnostics();
      Diagnostic first = diagnostics.get(0);
      assertEquals(diagnostics, restoredDiagnostics);
      assertThrows(UnsupportedOperationException.class, () -> restoredDiagnostics.add(first));
    }
  }

  @Test
  void aModelExceptionRejectsExcessiveSerializedDiagnostics() throws Exception {
    ByteArrayOutputStream bytes = new ByteArrayOutputStream();
    try (ObjectOutputStream output = new ExcessiveDiagnosticCountStream(bytes)) {
      output.writeObject(new ModelException("rejected", List.of()));
    }

    try (ObjectInputStream input =
        new ObjectInputStream(new ByteArrayInputStream(bytes.toByteArray()))) {
      InvalidObjectException failure =
          assertThrows(InvalidObjectException.class, input::readObject);
      assertEquals("too many diagnostics", failure.getMessage());
    }
  }

  private static final class ExcessiveDiagnosticCountStream extends ObjectOutputStream {
    private boolean replaceNextInt;

    ExcessiveDiagnosticCountStream(ByteArrayOutputStream output) throws IOException {
      super(output);
    }

    @Override
    public void defaultWriteObject() throws IOException {
      super.defaultWriteObject();
      replaceNextInt = true;
    }

    @Override
    public void writeInt(int value) throws IOException {
      if (replaceNextInt) {
        super.writeInt(Integer.MAX_VALUE);
        replaceNextInt = false;
      } else {
        super.writeInt(value);
      }
    }
  }

  @Test
  void capabilitiesNegotiateOnNames() {
    Capabilities capabilities =
        new Capabilities("dev", java.util.Set.of(Capabilities.QUERY, Capabilities.TYPE_FACTS));
    assertTrue(capabilities.has(Capabilities.QUERY));
    assertFalse(capabilities.has(Capabilities.CONVERT));
    capabilities.require(Capabilities.TYPE_FACTS);
    CapabilityException refused =
        assertThrows(CapabilityException.class, () -> capabilities.require(Capabilities.CONVERT));
    assertEquals(Capabilities.CONVERT, refused.capability());
    java.util.Set<String> names = capabilities.names();
    assertThrows(UnsupportedOperationException.class, () -> names.add("invented"));
  }

  @Test
  void optionsRejectWhatWouldFailLater() {
    ConnectionOptions.Builder builder = ConnectionOptions.builder();
    assertThrows(IllegalArgumentException.class, () -> builder.service("localhost", 0));
    assertThrows(IllegalArgumentException.class, () -> builder.expectedBinarySha256("not-a-digest"));
    assertThrows(
        IllegalArgumentException.class, () -> builder.requestTimeout(java.time.Duration.ZERO));
    ConnectionOptions options = ConnectionOptions.defaults();
    assertEquals(Encoding.PROTOBUF, options.encoding());
    assertTrue(options.autoStart());
    assertFalse(options.isolatedService());
    assertEquals(Optional.empty(), options.host());
  }

  @Test
  void refusingToStartWithoutAServiceIsReportedAtOpen() {
    ConnectionOptions options = ConnectionOptions.builder().autoStart(false).build();
    if (System.getenv(ConnectionOptions.SERVICE_ENV) != null) {
      return; // an external service is named in this environment, so opening would succeed
    }
    ServiceStartException refused =
        assertThrows(ServiceStartException.class, () -> Connection.open(options));
    assertTrue(refused.getMessage().contains("autoStart"));
  }
}
