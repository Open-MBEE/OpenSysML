package org.openmbee.opensysml;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertNotEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

import java.nio.file.Files;
import java.nio.file.Path;
import java.util.List;
import java.util.Optional;
import org.junit.jupiter.api.AfterAll;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInstance;

/** The v1 API against a real service this test starts. */
@TestInstance(TestInstance.Lifecycle.PER_CLASS)
class ApiIntegrationTest {

  private static final String VEHICLE =
      """
      package Demo {
        part def Engine { attribute power = 300.0; }
        part def Vehicle {
          attribute mass default = 1500.0;
          part engine : Engine;
        }
        part sedan : Vehicle { attribute :>> mass = 1200.0; }
      }
      """;

  private static Connection connection;

  @BeforeAll
  static void open() {
    connection = Connection.open(ServiceBinary.options().build());
  }

  @AfterAll
  static void close() {
    if (connection != null) {
      connection.close();
    }
  }

  @Test
  void theServiceAdvertisesTheCapabilitiesTheClientNegotiatesOn() {
    Capabilities capabilities = connection.capabilities();
    assertFalse(capabilities.serviceVersion().isBlank());
    assertTrue(capabilities.has(Capabilities.EVALUATE_SUBJECT));
    assertTrue(capabilities.has(Capabilities.TYPE_FACTS));
  }

  @Test
  void parsesInlineContent() {
    Model model = connection.parse(VEHICLE);
    assertFalse(model.hash().isBlank());
    assertTrue(model.root().isPresent());
    assertTrue(model.parseDiagnostics().isEmpty());
    assertEquals(List.of(), model.diagnostics());
  }

  @Test
  void parsesAFileTheServiceReads() throws Exception {
    Path file = Files.createTempFile("opensysml", ".sysml");
    try {
      Files.writeString(file, VEHICLE);
      Model model = connection.load(file);
      assertEquals("Demo", model.symbol("Demo").name());
    } finally {
      Files.deleteIfExists(file);
    }
  }

  @Test
  void evaluatesExpressionsAgainstDeclarationsAndAgainstObjects() {
    Model model = connection.parse(VEHICLE);
    assertEquals(new Value.IntegerValue(4), model.eval("2 + 2"));
    assertEquals(new Value.RealValue(1500.0), model.evalInContext("mass", "Demo::Vehicle"));
    assertEquals(new Value.RealValue(1200.0), model.evalWithSubject("mass", "Demo::sedan"));
  }

  @Test
  void anExpressionThatCannotBeEvaluatedIsAModelFailureRatherThanATransportOne() {
    Model model = connection.parse(VEHICLE);
    ModelException failed = assertThrows(ModelException.class, () -> model.eval("nosuchname + 1"));
    assertFalse(failed.getMessage().isBlank());
  }

  @Test
  void looksUpSymbols() {
    Model model = connection.parse(VEHICLE);
    Symbol vehicle = model.symbol("Demo::Vehicle");
    assertEquals("Demo::Vehicle", vehicle.id());
    assertEquals("Vehicle", vehicle.name());
    assertEquals("partDef", vehicle.kind());
    assertTrue(vehicle.childIds().contains("Demo::Vehicle::mass"));
    assertEquals(Optional.empty(), model.findSymbol("Demo::Missing"));
    assertThrows(ModelException.class, () -> model.symbol("Demo::Missing"));
  }

  @Test
  void instantiatesAnObjectAndItsFeatureValues() {
    Model model = connection.parse(VEHICLE);
    Instantiation instantiation = model.instantiate("Demo::sedan");
    assertEquals("Demo::sedan", instantiation.root().typeSymbolId());
    assertTrue(instantiation.reachable().size() >= 2);
    Instance.FeatureValue mass = instantiation.root().featureValues().get("mass");
    assertEquals(Optional.of(new Value.RealValue(1200.0)), mass.value());
    Instance.FeatureValue engine = instantiation.root().featureValues().get("engine");
    Value.InstanceReference reference = (Value.InstanceReference) engine.value().orElseThrow();
    assertEquals("Demo::Engine", instantiation.resolve(reference).orElseThrow().typeSymbolId());
  }

  private static final String COMPLEX =
      """
      package C {
        private import ScalarValues::*;
        private import ComplexFunctions::*;
        part def Signal {
          attribute z : Complex = rect(1.5, -2.0);
          attribute zs : Complex[2] = (rect(1.0, 2.0), rect(3.0, 4.0));
        }
      }
      """;

  @Test
  void aComplexNumberIsOneValueOverProtobufAndJson() {
    assertTrue(connection.capabilities().has(Capabilities.COMPLEX_VALUES));
    try (Connection json =
        Connection.open(ServiceBinary.options().encoding(Encoding.JSON).build())) {
      for (Connection each : List.of(connection, json)) {
        Model model = each.parse(COMPLEX);
        assertEquals(new Value.ComplexValue(1.5, -2.0), model.evalInContext("z", "C::Signal"));
        Instance signal = model.instantiate("C::Signal").root();
        assertEquals(
            Optional.of(new Value.ComplexValue(1.5, -2.0)),
            signal.featureValues().get("z").value());
        assertEquals(
            List.of(new Value.ComplexValue(1.0, 2.0), new Value.ComplexValue(3.0, 4.0)),
            signal.featureValues().get("zs").values());
      }
    }
  }

  private static final String STRUCTURED =
      """
      package S {
        private import ScalarValues::*;
        private import Collections::*;
        private import VectorValues::*;
        private import VectorFunctions::*;
        private import Quantities::*;
        private import SI::*;
        attribute grid : Array { :>> dimensions = (2, 3); :>> elements = (1, 2, 3, 4, 5, 6); }
        attribute v : CartesianVectorValue = VectorOf((3.0, 4.0));
        attribute d : VectorQuantityValue = VectorOf((3.0, 4.0)) [m];
      }
      """;

  @Test
  void anArrayAVectorAndAVectorQuantityArriveWholeOverProtobufAndJson() {
    assertTrue(connection.capabilities().has(Capabilities.STRUCTURED_VALUES));
    try (Connection json =
        Connection.open(ServiceBinary.options().encoding(Encoding.JSON).build())) {
      for (Connection each : List.of(connection, json)) {
        Model model = each.parse(STRUCTURED);
        assertEquals(
            new Value.ArrayValue(
                List.of(2L, 3L),
                List.of(
                    new Value.IntegerValue(1), new Value.IntegerValue(2), new Value.IntegerValue(3),
                    new Value.IntegerValue(4), new Value.IntegerValue(5), new Value.IntegerValue(6))),
            model.eval("S::grid"));
        assertEquals(
            new Value.VectorValue(List.of(new Value.RealValue(3.0), new Value.RealValue(4.0))),
            model.eval("S::v"));
        Value.VectorQuantityValue d = (Value.VectorQuantityValue) model.eval("S::d");
        assertEquals(Optional.of("m"), d.unit());
        assertEquals(List.of(3.0, 4.0), d.components().stream().map(Quantity::magnitude).toList());
        assertEquals(
            List.of(new Quantity.UnitFactor("SI::metre", 1.0)),
            d.components().get(0).reduction().orElseThrow().factors());
      }
    }
  }

  private static final String SET_AND_TENSOR =
      """
      package T {
        private import ScalarValues::*;
        private import Collections::*;
        private import Quantities::*;
        private import MeasurementReferences::*;
        private import SI::*;
        attribute s : Set { :>> elements = (3, 1, 2, 2, 3); }
        attribute none : Set { :>> elements = (); }
        attribute cubeRef : TensorMeasurementReference {
          :>> dimensions = (2, 2, 2);
          :>> mRefs = (m, m, m, m, m, m, m, m);
        }
        attribute cube : TensorQuantityValue =
          TensorCalculations::'['((1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0), cubeRef);
      }
      """;

  @Test
  void aSetAndARankThreeTensorArriveWholeOverProtobufAndJson() {
    assertTrue(connection.capabilities().has(Capabilities.SET_VALUES));
    assertTrue(connection.capabilities().has(Capabilities.TENSOR_VALUES));
    try (Connection json =
        Connection.open(ServiceBinary.options().encoding(Encoding.JSON).build())) {
      for (Connection each : List.of(connection, json)) {
        Model model = each.parse(SET_AND_TENSOR);
        Value.SetValue s = (Value.SetValue) model.eval("T::s.elements");
        assertEquals(
            new Value.SetValue(
                List.of(
                    new Value.IntegerValue(1), new Value.IntegerValue(2), new Value.IntegerValue(3))),
            s);
        assertEquals(List.of(1L, 2L, 3L), s.elements().stream().map(Value::asLong).toList());
        assertEquals(new Value.SetValue(List.of()), model.eval("T::none.elements"));
        Value.TensorQuantityValue cube = (Value.TensorQuantityValue) model.eval("T::cube");
        assertEquals(List.of(2L, 2L, 2L), cube.dimensions());
        assertEquals(Optional.of("m"), cube.unit());
        assertEquals(
            List.of(1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0),
            cube.components().stream().map(Quantity::magnitude).toList());
        assertEquals(6.0, cube.get(1, 0, 1).magnitude());
        assertEquals(
            List.of(new Quantity.UnitFactor("SI::metre", 1.0)),
            cube.get(1, 0, 1).reduction().orElseThrow().factors());
        Value.QuantityValue corner = (Value.QuantityValue) model.eval("T::cube#(2, 1, 2)");
        assertEquals(6.0, corner.quantity().magnitude());
        assertEquals(Optional.of("m"), corner.quantity().unit());
      }
    }
  }

  private static final String MEASUREMENT_REFS =
      """
      package M {
        private import ScalarValues::*;
        private import Quantities::*;
        private import MeasurementReferences::*;
        private import SI::*;
        attribute q : ISQ::LengthValue = 3 [km];
        attribute u : MeasurementUnit = m;
        attribute speed = m / s;
      }
      """;

  @Test
  void theServiceAdvertisesTheVerificationBodyVerdictsItReports() {
    assertTrue(connection.capabilities().has(Capabilities.VERIFICATION_VERDICTS));
  }

  @Test
  void theServiceAdvertisesTheScheduleOfItsExecutionRequests() {
    assertTrue(connection.capabilities().has(Capabilities.SCHEDULE));
  }

  @Test
  void theServiceAdvertisesTheFinalClockInstantOfItsExecutionResponses() {
    assertTrue(connection.capabilities().has(Capabilities.FINAL_TIME));
  }

  @Test
  void aBareMeasurementReferenceArrivesWithItsReductionAndDeclarationOverProtobufAndJson() {
    assertTrue(connection.capabilities().has(Capabilities.MEASUREMENT_REFS));
    try (Connection json =
        Connection.open(ServiceBinary.options().encoding(Encoding.JSON).build())) {
      for (Connection each : List.of(connection, json)) {
        Model model = each.parse(MEASUREMENT_REFS);
        assertEquals(
            new Value.MeasurementRefValue(
                "m",
                new Quantity.UnitTerm(
                    1.0, 1.0, List.of(new Quantity.UnitFactor("SI::metre", 1.0))),
                Optional.of("SI::metre")),
            model.eval("M::u"));
        Value.MeasurementRefValue km = (Value.MeasurementRefValue) model.eval("M::q.mRef");
        assertEquals("km", km.unit());
        assertEquals(Optional.of("SI::kilometre"), km.unitId());
        assertEquals(1000.0, km.reduction().scaleNumerator());
        Value.MeasurementRefValue speed = (Value.MeasurementRefValue) model.eval("M::speed");
        assertEquals(Optional.empty(), speed.unitId());
        assertEquals(
            List.of(
                new Quantity.UnitFactor("SI::metre", 1.0),
                new Quantity.UnitFactor("SI::second", -1.0)),
            speed.reduction().factors());
      }
    }
  }

  private static final String FUNCTIONS =
      """
      package Demo {
        private import ScalarValues::*;
        calc def Sq { in v : Real; return : Real = v * v; }
        calc def Fn { in calc f { in v : Real; return : Real; } in a : Real; return : Real = f(a); }
        calc def Identity { in calc f { in v : Real; return : Real; } return r = f; }
        attribute pick = Identity(Sq);
        attribute nine = Fn(Sq, 3.0);
        part def Scaler {
          attribute k : Real = 2.0;
          calc scale { in x : Real; return : Real = x * k; }
        }
        part holder : Scaler;
        attribute scaler = holder.scale;
      }
      """;

  @Test
  void aCalcHeldAsAValueArrivesAsTheFunctionItNamesOverProtobufAndJson() {
    assertTrue(connection.capabilities().has(Capabilities.FUNCTION_VALUES));
    try (Connection json =
        Connection.open(ServiceBinary.options().encoding(Encoding.JSON).build())) {
      for (Connection each : List.of(connection, json)) {
        Model model = each.parse(FUNCTIONS);
        assertEquals(
            new Value.FunctionValue("Demo::Sq", Optional.empty()), model.eval("Demo::pick"));
        assertEquals(new Value.RealValue(9.0), model.eval("Demo::nine"));
        Value.FunctionValue scale = (Value.FunctionValue) model.eval("Demo::scaler");
        assertEquals("Demo::Scaler::scale", scale.calcId());
        assertTrue(scale.selfId().orElseThrow() > 0);
      }
    }
  }

  @Test
  void aModelTheServiceDoesNotHoldIsRefused() {
    Model absent = connection.model("sha256:0000000000000000");
    OpenSysMLException refused =
        assertThrows(OpenSysMLException.class, () -> absent.symbol("Demo::Vehicle"));
    if (refused instanceof ServiceException service) {
      assertEquals(StatusCode.NOT_FOUND, service.status());
    }
  }

  @Test
  void aParseThatFindsErrorsReportsThemAsDiagnosticsRatherThanARefusal() {
    Model model = connection.parse("part def { { {");
    List<Diagnostic> diagnostics = model.parseDiagnostics();
    assertFalse(diagnostics.isEmpty());
    assertEquals(Diagnostic.Severity.ERROR, diagnostics.get(0).severity());
    assertEquals(diagnostics, model.diagnostics());
  }

  @Test
  void theServiceAdvertisesTheDiagnosticCodesItPopulates() {
    assertTrue(connection.capabilities().has(Capabilities.DIAGNOSTIC_CODES));
    Model model = connection.parse("package P { part def W { part hub : Missing; } }");
    assertTrue(model.diagnostics().stream().anyMatch(d -> "unresolved".equals(d.code())));
  }

  @Test
  void aJsonBodyAnswersWhatAProtobufBodyAnswers() {
    try (Connection json =
        Connection.open(ServiceBinary.options().encoding(Encoding.JSON).build())) {
      assertEquals(connection.address(), json.address(), "the private service is shared");
      Model model = json.parse(VEHICLE);
      assertEquals(new Value.RealValue(1200.0), model.evalWithSubject("mass", "Demo::sedan"));
      assertEquals(connection.capabilities(), json.capabilities());
    }
  }

  @Test
  void aModelHashOutlivesTheConnectionThatParsedIt() {
    String hash;
    try (Connection first = Connection.open(ServiceBinary.options().build())) {
      hash = first.parse(VEHICLE).hash();
    }
    assertEquals(new Value.IntegerValue(4), connection.model(hash).eval("2 + 2"));
  }

  @Test
  void twoModelsAreToldApartByHash() {
    assertNotEquals(connection.parse(VEHICLE).hash(), connection.parse("package Other {}").hash());
  }

  @Test
  void strictConformanceIsCapabilityGated() {
    ParseOptions strict = ParseOptions.defaults().withStrictConformance(true);
    if (connection.capabilities().has(Capabilities.STRICT_CONFORMANCE)) {
      assertFalse(connection.parse(VEHICLE, strict).hash().isBlank());
    } else {
      assertThrows(CapabilityException.class, () -> connection.parse(VEHICLE, strict));
    }
  }

  @Test
  void aClosedConnectionRefusesCalls() {
    Connection closed = Connection.open(ServiceBinary.options().build());
    closed.close();
    closed.close(); // idempotent
    assertThrows(IllegalStateException.class, () -> closed.parse(VEHICLE));
  }
}
