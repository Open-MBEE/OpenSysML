package org.openmbee.opensysml;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertInstanceOf;
import static org.junit.jupiter.api.Assertions.assertNotEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

import com.sun.net.httpserver.HttpServer;
import java.io.OutputStream;
import java.net.InetSocketAddress;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import org.junit.jupiter.api.AfterAll;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInstance;
import org.openmbee.opensysml.proto.CaseEvaluation;
import org.openmbee.opensysml.proto.RunAnalysisRequest;
import org.openmbee.opensysml.proto.RunAnalysisResponse;
import org.openmbee.opensysml.proto.RunSweepRequest;
import org.openmbee.opensysml.proto.RunSweepResponse;
import org.openmbee.opensysml.proto.SweepRange;
import org.openmbee.opensysml.proto.SweepRow;

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
    assertTrue(capabilities.has(Capabilities.CONNECTION_AUTHORING));
    assertTrue(capabilities.has(Capabilities.TRANSITION_AUTHORING));
    assertTrue(capabilities.has(Capabilities.SEQUENCE_AUTHORING));
    assertTrue(capabilities.has(Capabilities.ACTION_BODY_STATEMENT_AUTHORING));
    assertTrue(capabilities.has(Capabilities.IMPORT_AUTHORING));
    assertTrue(capabilities.has(Capabilities.DOCUMENTATION_AUTHORING));
    assertTrue(capabilities.has(Capabilities.COMMENT_AUTHORING));
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

  private static final String METAOBJECTS =
      """
      package Demo {
        private import ScalarValues::*;
        metadata def Safety { attribute level : Integer = 2; }
        part def Vehicle { attribute mass : Real; }
        part seatBelt : Vehicle { @Safety { level = 4; } }
        attribute asFeature [*] = seatBelt meta KerML::Feature;
        attribute everything [*] = seatBelt.metadata;
        attribute notADefinition [*] = seatBelt meta SysML::PartDefinition;
        attribute belt : String = (seatBelt meta KerML::Feature)#(1).declaredName;
      }
      """;

  @Test
  void aMetaCastArrivesAsTheElementUnderItsOwnMetaclassAfterItsAnnotations() {
    assertTrue(connection.capabilities().has(Capabilities.METAOBJECT_VALUES));
    Value.MetaobjectValue seatBelt =
        new Value.MetaobjectValue("Demo::seatBelt", "SysML::Systems::PartUsage");
    try (Connection json =
        Connection.open(ServiceBinary.options().encoding(Encoding.JSON).build())) {
      for (Connection each : List.of(connection, json)) {
        Model model = each.parse(METAOBJECTS);
        Value.Sequence asFeature = (Value.Sequence) model.eval("Demo::asFeature");
        assertEquals(List.of(seatBelt), asFeature.elements());
        assertEquals(
            "SysML::Systems::PartUsage",
            ((Value.MetaobjectValue) asFeature.elements().get(0)).metaclassId());
        assertEquals(new Value.Sequence(List.of()), model.eval("Demo::notADefinition"));
        Value.Sequence everything = (Value.Sequence) model.eval("Demo::everything");
        assertEquals(2, everything.elements().size());
        assertInstanceOf(Value.InstanceReference.class, everything.elements().get(0));
        assertEquals(seatBelt, everything.elements().get(1));
        assertEquals(new Value.StringValue("seatBelt"), model.eval("Demo::belt"));
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

  private static final String TRADE_STUDY =
      """
      package Trade {
        private import ScalarValues::*;
        private import TradeStudies::*;
        part def Engine { attribute mass : Real; attribute cylinders : Integer; }
        part a : Engine { attribute :>> mass = 30.0; attribute :>> cylinders = 6; }
        part b : Engine { attribute :>> mass = 10.0; attribute :>> cylinders = 4; }
        part c : Engine { attribute :>> mass = 10.0; attribute :>> cylinders = 0; }
        analysis lightest : TradeStudy {
          subject : Engine[1..*] = (a, b, c);
          objective : MinimizeObjective;
          calc :>> evaluationFunction {
            in part e :>> alternative : Engine;
            return :>> result : Real = e.mass;
          }
          return part :>> selectedAlternative : Engine;
        }
        analysis perOffset : TradeStudy {
          subject : Engine[1..*] = (a, b);
          in attribute offset : Integer;
          objective : MinimizeObjective;
          calc :>> evaluationFunction {
            in part e :>> alternative : Engine;
            return :>> result : Real = e.mass / (e.cylinders - offset);
          }
          return part :>> selectedAlternative : Engine;
        }
      }
      """;

  @Test
  void theServiceAdvertisesTheCaseEvaluationsItReports() {
    assertTrue(connection.capabilities().has(Capabilities.CASE_EVALUATIONS));
  }

  @Test
  void aTradeStudyArrivesWithEachAlternativesEvaluationTheSelectedOneAndTheTieMarked() {
    try (Connection json =
        Connection.open(ServiceBinary.options().encoding(Encoding.JSON).build())) {
      for (Connection each : List.of(connection, json)) {
        Model model = each.parse(TRADE_STUDY);
        RunAnalysisResponse response =
            each.call(
                "RunAnalysis",
                RunAnalysisRequest.newBuilder()
                    .setModelHash(model.hash())
                    .setSymbolId("Trade::lightest")
                    .build(),
                RunAnalysisResponse.getDefaultInstance());
        assertEquals("", response.getError());
        assertEquals(1, response.getOutputsCount());
        long selected = response.getOutputs(0).getValue().getInstanceId();
        assertEquals(1, response.getVerdictsCount());
        assertEquals("tradeStudyObjective", response.getVerdicts(0).getElement());
        assertTrue(response.getVerdicts(0).getHolds());

        List<CaseEvaluation> evaluations = response.getEvaluationsList();
        assertEquals(
            List.of("Trade::a", "Trade::b", "Trade::c"),
            evaluations.stream()
                .map(e -> typeOf(response.getInstancesList(), e.getArguments(0).getInstanceId()))
                .toList());
        assertEquals(
            List.of(30.0, 10.0, 10.0),
            evaluations.stream().map(e -> e.getResult().getRealValue()).toList());
        assertEquals(
            List.of(false, true, false),
            evaluations.stream().map(CaseEvaluation::getSelected).toList());
        assertEquals(
            List.of(false, false, true),
            evaluations.stream().map(CaseEvaluation::getTied).toList());
        assertEquals(selected, evaluations.get(1).getArguments(0).getInstanceId());
        evaluations.forEach(
            e -> assertEquals("Trade::lightest::evaluationFunction", e.getFunctionId()));
      }
    }
  }

  @Test
  void aSweptTradeStudyCarriesEachRowsEvaluationsAFailedRowKeepingThoseItMade() {
    Model model = connection.parse(TRADE_STUDY);
    RunSweepResponse response =
        connection.call(
            "RunSweep",
            RunSweepRequest.newBuilder()
                .setModelHash(model.hash())
                .setSymbolId("Trade::perOffset")
                .addRanges(
                    SweepRange.newBuilder()
                        .setParameter("offset")
                        .setStart(
                            org.openmbee.opensysml.proto.Value.newBuilder().setIntValue(3))
                        .setEnd(org.openmbee.opensysml.proto.Value.newBuilder().setIntValue(4)))
                .build(),
            RunSweepResponse.getDefaultInstance());
    assertEquals("", response.getError());
    assertEquals(2, response.getRowsCount());

    SweepRow ok = response.getRows(0);
    assertEquals("", ok.getError());
    assertEquals(1, ok.getOutputsCount());
    assertEquals(
        List.of(10.0, 10.0),
        ok.getEvaluationsList().stream().map(e -> e.getResult().getRealValue()).toList());
    assertEquals(
        List.of(true, false),
        ok.getEvaluationsList().stream().map(CaseEvaluation::getSelected).toList());
    assertEquals(
        List.of(false, true),
        ok.getEvaluationsList().stream().map(CaseEvaluation::getTied).toList());

    SweepRow failed = response.getRows(1);
    assertTrue(failed.getError().contains("division by zero"));
    assertEquals(0, failed.getOutputsCount());
    assertEquals(1, failed.getVerdictsCount());
    assertFalse(failed.getVerdicts(0).getHolds());
    assertTrue(failed.getVerdicts(0).getError().contains("division by zero"));
    assertEquals(2, failed.getEvaluationsCount());
    assertEquals(15.0, failed.getEvaluations(0).getResult().getRealValue());
    assertEquals("", failed.getEvaluations(0).getError());
    assertFalse(failed.getEvaluations(1).hasResult());
    assertTrue(failed.getEvaluations(1).getError().contains("division by zero"));
    long failedAlternative = failed.getEvaluations(1).getArguments(0).getInstanceId();
    assertEquals("Trade::b", typeOf(response.getInstancesList(), failedAlternative));
    assertFalse(failed.getEvaluationsList().stream().anyMatch(CaseEvaluation::getSelected));
  }

  private static String typeOf(List<org.openmbee.opensysml.proto.Instance> instances, long id) {
    return instances.stream()
        .filter(inst -> inst.getId() == id)
        .map(org.openmbee.opensysml.proto.Instance::getTypeSymbolId)
        .findFirst()
        .orElseThrow();
  }

  @Test
  void theServiceAdvertisesTheScheduleOfItsExecutionRequests() {
    assertTrue(connection.capabilities().has(Capabilities.SCHEDULE));
  }

  @Test
  void theServiceAdvertisesTheExploreSchedule() {
    assertTrue(connection.capabilities().has(Capabilities.SCHEDULE_EXPLORE));
  }

  @Test
  void theServiceAdvertisesTheObjectABehaviorIsPerformedBy() {
    assertTrue(connection.capabilities().has(Capabilities.PERFORMER));
  }

  @Test
  void theServiceAdvertisesItsAnalysisEngines() {
    assertTrue(connection.capabilities().has(Capabilities.ENGINES));
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

  private static final String BEHAVIOR =
      """
      package Test {
        private import ScalarValues::*;
        action addFive {
          attribute result : Integer = 0;
          first start;
          action inner { assign result := result + 5; }
          done;
          succession first start then inner;
          succession first inner then done;
        }
        action noStart {
          action a;
          action b;
          succession first a then b;
          succession first b then a;
        }
        action race {
          attribute x : Integer = 0;
          first start;
          fork split;
          action a { assign x := 1; }
          action b { assign x := 2; }
          action c { assign x := 3; }
          join sync;
          done;
          succession first start then split;
          succession first split then a;
          succession first split then b;
          succession first split then c;
          succession first a then sync;
          succession first b then sync;
          succession first c then sync;
          succession first sync then done;
        }
        state Machine {
          entry; then init;
          state init;
          state Running;
          succession first init then Running;
          succession first Running then done;
        }
      }
      """;

  @Test
  void anActionRunsWithItsInputsAndReportsItsAttributes() {
    Model model = connection.parse(BEHAVIOR);
    ActionRun run = model.executeAction("Test::addFive");
    assertEquals(new Value.IntegerValue(5), run.outputs().get("result"));
    assertEquals(
        connection.capabilities().has(Capabilities.FINAL_TIME), run.finalTime().isPresent());

    ActionRun seeded =
        model.executeAction("Test::addFive", Map.of("result", new Value.IntegerValue(10)));
    assertEquals(new Value.IntegerValue(15), seeded.outputs().get("result"));

    ActionRun declared =
        model.executeAction(
            "Test::race", Map.of(), ExecutionOptions.defaults().withSchedule("declared"));
    assertEquals(new Value.IntegerValue(3), declared.outputs().get("x"));
    assertThrows(
        IllegalArgumentException.class,
        () ->
            model.executeAction(
                "Test::addFive", Map.of(), ExecutionOptions.defaults().withTrace()));
    assertThrows(
        IllegalArgumentException.class,
        () ->
            model.exploreAction(
                "Test::race", Map.of(), ExecutionOptions.defaults().withTrace()));
  }

  @Test
  void anActionThatCannotStartIsAModelFailureAndABadScheduleIsRefused() {
    Model model = connection.parse(BEHAVIOR);
    ModelException failed =
        assertThrows(ModelException.class, () -> model.executeAction("Test::noStart"));
    assertFalse(failed.getMessage().isBlank());
    ExecutionOptions seeded = ExecutionOptions.defaults().withSchedule("seed:abc");
    ServiceException refused =
        assertThrows(
            ServiceException.class, () -> model.executeAction("Test::race", Map.of(), seeded));
    assertEquals(StatusCode.INVALID_ARGUMENT, refused.status());
    assertTrue(refused.getMessage().contains("seed:abc"));
    ExecutionOptions declared = ExecutionOptions.defaults().withSchedule("declared");
    assertThrows(
        IllegalArgumentException.class, () -> model.exploreAction("Test::race", Map.of(), declared));
  }

  @Test
  void exploringAnActionReachesEveryOutcomeWithItsWitness() {
    Model model = connection.parse(BEHAVIOR);
    Exploration exploration = model.exploreAction("Test::race");
    assertTrue(exploration.complete());
    assertEquals(6, exploration.runs());
    assertEquals(List.of(), exploration.budgetsHit());
    assertEquals("complete (6 runs)", exploration.status());
    assertEquals(
        List.of(new Value.IntegerValue(1), new Value.IntegerValue(2), new Value.IntegerValue(3)),
        exploration.outcomes().stream().map(o -> o.outputs().get("x")).toList());
    for (Outcome outcome : exploration.outcomes()) {
      assertTrue(outcome.completed());
      assertEquals(2, outcome.linearizations());
      assertFalse(outcome.witness().isEmpty());
    }

    Exploration bounded =
        model.exploreAction(
            "Test::race", Map.of(), ExecutionOptions.defaults().withSchedule("explore:runs=2"));
    assertFalse(bounded.complete());
    assertEquals(List.of("runs"), bounded.budgetsHit());
    assertEquals(2, bounded.runsBudget());
    assertTrue(bounded.status().startsWith("incomplete: runs budget 2"));
  }

  @Test
  void aStateMachineReportsTheStatesItVisitedAndExploresToItsFinalState() {
    Model model = connection.parse(BEHAVIOR);
    StateRun run = model.executeState("Test::Machine", List.of());
    assertEquals(List.of("init", "Running", "done"), run.statesVisited());
    assertEquals(Optional.of("done"), run.finalState());
    assertTrue(run.trace().isEmpty());
    StateRun traced =
        model.executeState("Test::Machine", List.of(), ExecutionOptions.defaults().withTrace());
    assertEquals("entry", traced.trace().get(0).kind());
    assertEquals(0, traced.traceDropped());

    Exploration exploration = model.exploreState("Test::Machine", List.of());
    assertTrue(exploration.complete());
    assertEquals(1, exploration.outcomes().size());
    assertEquals(Optional.of("done"), exploration.outcomes().get(0).finalState());
    assertEquals(
        List.of("init", "Running", "done"), exploration.outcomes().get(0).statesVisited());
    assertThrows(
        IllegalArgumentException.class,
        () ->
            model.exploreState(
                "Test::Machine", List.of(), ExecutionOptions.defaults().withTrace()));

    assertThrows(ModelException.class, () -> model.executeState("Test::NoMachine", List.of()));
  }

  @Test
  void aFailedTracedStateRunKeepsItsPartialTraceOnTheModelException() throws Exception {
    HttpServer server = HttpServer.create(new InetSocketAddress("127.0.0.1", 0), 0);
    server.createContext(
        "/",
        exchange -> {
          exchange.getRequestBody().readAllBytes();
          byte[] body;
          if (exchange.getRequestURI().getPath().endsWith("/GetServerInfo")) {
            body =
                org.openmbee.opensysml.proto.ServerInfoResponse.newBuilder()
                    .setVersion("test")
                    .addCapabilities(Capabilities.STATE_TRACE)
                    .build()
                    .toByteArray();
          } else {
            body =
                org.openmbee.opensysml.proto.ExecuteStateResponse.newBuilder()
                    .setError("state machine failed")
                    .addTrace(
                        org.openmbee.opensysml.proto.DocumentEvent.newBuilder()
                            .setKind("entry")
                            .setTime(
                                org.openmbee.opensysml.proto.DocumentValue.newBuilder()
                                    .setRealValue(1.5))
                            .setState("active")
                            .setText("enter: active"))
                    .setTraceDropped(2)
                    .build()
                    .toByteArray();
          }
          exchange.getResponseHeaders().add("Content-Type", "application/proto");
          exchange.sendResponseHeaders(200, body.length);
          try (OutputStream output = exchange.getResponseBody()) {
            output.write(body);
          }
        });
    server.start();
    try (Connection fake =
        Connection.open(
            ConnectionOptions.builder()
                .service("127.0.0.1", server.getAddress().getPort())
                .build())) {
      Model model = new Model(fake, "hash", List.of(), List.of());
      ModelException failure =
          assertThrows(
              ModelException.class,
              () ->
                  model.executeState(
                      "Trace::Machine", List.of(), ExecutionOptions.defaults().withTrace()));
      assertEquals("state machine failed", failure.getMessage());
      assertEquals(1, failure.trace().size());
      assertEquals("entry", failure.trace().get(0).kind());
      assertEquals("active", failure.trace().get(0).state());
      assertEquals(2, failure.traceDropped());
    } finally {
      server.stop(0);
    }
  }

  private static final String VERIFICATION =
      """
      package Demo {
        part def Vehicle {
          attribute mass default = 1500.0;
          constraint massPositive { mass > 0.0 }
          constraint massLight { mass < 100.0 }
          requirement lightEnough { require constraint { mass < 2000.0 } }
          requirement tiny { require constraint { mass < 10.0 } }
        }
        requirement def MassLimit {
          subject vehicle : Vehicle;
          attribute maxMass;
          require constraint { vehicle.mass <= maxMass }
        }
        requirement massLimit : MassLimit { attribute :>> maxMass = 2000.0; }
        requirement massTiny : MassLimit { attribute :>> maxMass = 10.0; }
        part sedan : Vehicle { attribute :>> mass = 1200.0; }
        part analysis {
          assert satisfy massLimit by sedan;
          assert satisfy massTiny by sedan;
        }
        calc add { in x; in y; x + y }
      }
      """;

  @Test
  void aConstraintIsVerifiedAgainstDeclaredValuesOrAnObjectAndAFalseAnswerIsNotAFailure() {
    Model model = connection.parse(VERIFICATION);
    Verification holding = model.verifyConstraint("Demo::Vehicle::massPositive");
    assertTrue(holding.holds());
    assertTrue(holding.verdict().decided());
    assertEquals(Optional.empty(), holding.subject());

    Verification violated = model.verifyConstraint("Demo::Vehicle::massLight");
    assertFalse(violated.holds());
    assertTrue(violated.verdict().violated());
    assertTrue(violated.verdict().condition().isPresent());
    assertEquals(Optional.empty(), violated.verdict().error());

    Verification about = model.verifyConstraint("Demo::Vehicle::massPositive", "Demo::sedan");
    assertTrue(about.holds());
    assertEquals("Demo::sedan", about.subject().orElseThrow().typeSymbolId());

    Verification wrongKind = model.verifyConstraint("Demo::sedan");
    assertFalse(wrongKind.verdict().decided());
    assertFalse(wrongKind.holds());
    assertEquals(FailureReason.WRONG_KIND, wrongKind.verdict().failureReason());
    assertFalse(wrongKind.verdict().error().orElseThrow().isEmpty());
  }

  @Test
  void requirementsAndSatisfactionsReportEachVerdict() {
    Model model = connection.parse(VERIFICATION);
    assertTrue(model.verifyRequirement("Demo::Vehicle::lightEnough").holds());
    Verification tiny = model.verifyRequirement("Demo::Vehicle::tiny");
    assertTrue(tiny.verdict().violated());

    Satisfaction all = model.verifySatisfaction();
    assertEquals(2, all.verdicts().size());
    assertFalse(all.holds());
    assertEquals(1, all.violated().size());
    assertEquals(List.of(), all.undecided());
    assertEquals(Optional.of("Demo::massTiny"), all.violated().get(0).requirementId());

    Satisfaction scoped = model.verifySatisfaction("Demo::analysis");
    assertEquals(2, scoped.verdicts().size());

    Validation validation = model.validateInstance("Demo::sedan");
    assertFalse(validation.holds());
    assertEquals("Demo::sedan", validation.root().orElseThrow().typeSymbolId());
    assertTrue(validation.verdicts().size() >= 2);
    assertThrows(ModelException.class, () -> model.validateInstance("Demo::nosuch"));
    ModelException wrongKind =
        assertThrows(ModelException.class, () -> model.validateInstance("Demo"));
    assertEquals(FailureReason.WRONG_KIND, wrongKind.failureReason());
  }

  private static final String VERIFICATION_CASES =
      """
      package Demo {
        private import ScalarValues::*;
        part def Widget { attribute m : Integer default = 0; }
        part good : Widget;
        requirement def Zeroed {
          subject w : Widget;
          require constraint { w.m == 0 }
        }
        requirement zeroed : Zeroed { subject w = good; }
        requirement bounded : Zeroed { subject w = good; }
        verification def ZeroCheck {
          subject w : Widget;
          objective { verify zeroed; }
          VerificationCases::PassIf(w.m == 0)
        }
        verification def BoundCheck {
          subject w : Widget;
          objective { verify bounded; }
          VerificationCases::PassIf(w.m == 1)
        }
        verification checkZero : ZeroCheck { subject w = good; }
        verification checkBound : BoundCheck { subject w = good; }
        part checks {
          assert satisfy zeroed by good;
          assert satisfy bounded by good;
        }
      }
      """;

  @Test
  void theVerificationCasesOfARequirementReportBesideItsVerdict() {
    Model model = connection.parse(VERIFICATION_CASES);
    Verification bounded = model.verifyRequirement("Demo::bounded");
    assertTrue(bounded.holds());
    assertEquals(1, bounded.verifications().size());
    VerificationVerdict check = bounded.verifications().get(0);
    assertEquals("Demo::checkBound", check.caseId());
    assertEquals(VerificationVerdict.FAIL, check.kind());
    assertFalse(check.passed());
    assertEquals(Optional.of("Demo::bounded"), check.requirementId());

    Satisfaction checks = model.verifySatisfaction("Demo::checks");
    assertTrue(checks.holds());
    assertEquals(VerificationVerdict.PASS, checks.verificationsOf("Demo::zeroed").get(0).kind());
    assertEquals(VerificationVerdict.FAIL, checks.verificationsOf("Demo::bounded").get(0).kind());
  }

  @Test
  void aCalcIsEvaluatedWithPositionalArgumentsAndTheWrongKindIsAModelFailure() {
    Model model = connection.parse(VERIFICATION);
    Calculation sum =
        model.evaluateCalc(
            "Demo::add", List.of(new Value.IntegerValue(2), new Value.IntegerValue(3)));
    assertEquals(Optional.of(new Value.IntegerValue(5)), sum.value());
    assertEquals(Optional.of(new Value.IntegerValue(5)), sum.result());
    ModelException wrongKind =
        assertThrows(ModelException.class, () -> model.evaluateCalc("Demo::sedan", List.of()));
    assertEquals(FailureReason.WRONG_KIND, wrongKind.failureReason());
  }

  @Test
  void aTradeStudyArrivesThroughThePublicApiWithItsSelectedAlternative() {
    Model model = connection.parse(TRADE_STUDY);
    Analysis analysis = model.runAnalysis("Trade::lightest");
    assertTrue(analysis.holds());
    assertEquals("tradeStudyObjective", analysis.objective().orElseThrow().element());
    Value selected = analysis.outputs().get("selectedAlternative");
    assertInstanceOf(Value.InstanceReference.class, selected);
    assertEquals(
        "Trade::b",
        analysis.resolve((Value.InstanceReference) selected).orElseThrow().typeSymbolId());
    assertEquals(
        List.of("Trade::a", "Trade::b", "Trade::c"),
        analysis.evaluations().stream()
            .map(e -> analysis.resolve((Value.InstanceReference) e.arguments().get(0)))
            .map(i -> i.orElseThrow().typeSymbolId())
            .toList());
    assertEquals(
        List.of(false, true, false),
        analysis.evaluations().stream().map(org.openmbee.opensysml.CaseEvaluation::selected).toList());
    assertEquals(
        List.of(false, false, true),
        analysis.evaluations().stream().map(org.openmbee.opensysml.CaseEvaluation::tied).toList());
    assertEquals(analysis.evaluations().get(1), analysis.selected().orElseThrow());
  }

  @Test
  void anAnalysisBindsItsArgumentsAndAFailedRunKeepsWhatItLeft() {
    Model model = connection.parse(TRADE_STUDY);
    Analysis three =
        model.runAnalysis(
            "Trade::perOffset",
            AnalysisOptions.defaults().withNamedArguments(Map.of("offset", new Value.IntegerValue(3))));
    assertEquals(
        List.of(Optional.of(new Value.RealValue(10.0)), Optional.of(new Value.RealValue(10.0))),
        three.evaluations().stream().map(org.openmbee.opensysml.CaseEvaluation::result).toList());

    AnalysisOptions arguments =
        AnalysisOptions.defaults().withArguments(List.of(new Value.IntegerValue(4)));
    AnalysisException failed =
        assertThrows(AnalysisException.class, () -> model.runAnalysis("Trade::perOffset", arguments));
    assertTrue(failed.getMessage().contains("division by zero"));
    assertEquals(FailureReason.EVALUATION, failed.failureReason());
    Analysis partial = failed.partial().orElseThrow();
    assertEquals(2, partial.evaluations().size());
    assertEquals(Optional.of(new Value.RealValue(15.0)), partial.evaluations().get(0).result());
    assertTrue(partial.evaluations().get(1).error().orElseThrow().contains("division by zero"));
    assertFalse(partial.holds());
    assertEquals(Optional.empty(), partial.selected());

    ModelException wrongKind =
        assertThrows(ModelException.class, () -> model.runAnalysis("Trade::a"));
    assertEquals(FailureReason.WRONG_KIND, wrongKind.failureReason());
    assertFalse(wrongKind instanceof AnalysisException);
  }

  @Test
  void theExploreEngineRunsNoSingleAnalysis() {
    Model model = connection.parse(TRADE_STUDY);
    if (!connection.capabilities().has(Capabilities.SCHEDULE_EXPLORE)) {
      assertThrows(CapabilityException.class, () -> model.withEngine("explore"));
      return;
    }
    Model exploring = model.withEngine("explore");
    IllegalArgumentException refused =
        assertThrows(
            IllegalArgumentException.class, () -> exploring.runAnalysis("Trade::lightest"));
    assertEquals("engine explore answers every outcome; use exploreAnalysis", refused.getMessage());
  }

  private static final String QUERY =
      """
      package Demo {
        abstract part def Vehicle { attribute mass; }
        part def Wheel;
        part vehicle : Vehicle {
          part wheels : Wheel[4];
          attribute vin;
        }
        part spare : Wheel;
      }
      """;

  @Test
  void aQuerySelectsElementsByTypeScopeAndProperty() {
    Model model = connection.parse(QUERY);
    List<String> all = model.query(Query.all()).stream().map(QueryElement::id).toList();
    assertTrue(all.containsAll(List.of("Demo", "Demo::Vehicle", "Demo::vehicle::wheels", "Demo::spare")));

    List<QueryElement> parts =
        model.query(Query.all().where(Condition.equalTo("@type", List.of("PartUsage"))));
    assertEquals(
        List.of("Demo::spare", "Demo::vehicle", "Demo::vehicle::wheels"),
        parts.stream().map(QueryElement::id).sorted().toList());
    parts.forEach(e -> assertEquals("PartUsage", e.type()));

    List<QueryElement> scoped = model.query(Query.all().withScope(List.of("Demo::vehicle")));
    assertEquals(3, scoped.size());

    List<QueryElement> selected =
        model.query(
            Query.all()
                .withSelect(List.of("name", "owner"))
                .where(Condition.equalTo("qualifiedName", List.of("Demo::vehicle::wheels"))));
    assertEquals(
        List.of(
            new QueryElement(
                "Demo::vehicle::wheels",
                "PartUsage",
                Map.of("name", "wheels", "owner", "Demo::vehicle"))),
        selected);

    assertEquals(
        3, model.queryOslc("oslc.where=rdf:type=\"PartUsage\"&oslc.select=sysml:name").size());
    Query missingScope = Query.all().withScope(List.of("Demo::Missing"));
    ServiceException refused =
        assertThrows(ServiceException.class, () -> model.query(missingScope));
    assertEquals(StatusCode.INVALID_ARGUMENT, refused.status());
  }

  @Test
  void theServiceListsItsEnginesAndAModelCanBeBoundToOne() {
    List<EngineInfo> engines = connection.listEngines();
    assertFalse(engines.isEmpty());
    engines.forEach(e -> assertFalse(e.name().isBlank()));
    Model model = connection.parse(VERIFICATION);
    Model bound = model.withEngine(Standing.ENGINE_AUTO);
    assertEquals(Optional.of(Standing.ENGINE_AUTO), bound.engine());
    assertEquals(model.hash(), bound.hash());
    Verification holding = bound.verifyConstraint("Demo::Vehicle::massPositive");
    assertTrue(holding.holds());
  }

  private static Path v1Fixture(String name) {
    return Path.of(System.getProperty("user.dir"))
        .resolve("../../../tests/migrate/testdata/xmi")
        .resolve(name)
        .normalize();
  }

  private static Path fixture(String name) {
    return Path.of(System.getProperty("user.dir"))
        .resolve("../../../conformance/fixtures")
        .normalize()
        .resolve(name);
  }

  @Test
  void parseSourcesParsesSeveralDocumentsAsOneModel() throws Exception {
    Model model =
        connection.parseSources(
            List.of(
                SourceDocument.inline(
                    "engine_library.sysml",
                    Files.readString(fixture("engine_library.sysml"))),
                SourceDocument.inline(
                    "engine_user.sysml", Files.readString(fixture("engine_user.sysml")))));
    assertEquals(2, model.roots().size());
    assertTrue(model.root().isPresent());
    assertEquals("Engine", model.symbol("EngineLibrary::Engine").name());
    assertEquals("Car", model.symbol("EngineUser::Car").name());
  }

  @Test
  void parseSourcesRefusesTwoDocumentsOfOneName() throws Exception {
    String librarySource = Files.readString(fixture("engine_library.sysml"));
    String userSource = Files.readString(fixture("engine_user.sysml"));
    List<SourceDocument> documents =
        List.of(
            SourceDocument.inline("same.sysml", librarySource),
            SourceDocument.inline("same.sysml", userSource));
    ServiceException refused =
        assertThrows(ServiceException.class, () -> connection.parseSources(documents));
    assertEquals(StatusCode.INVALID_ARGUMENT, refused.status());
  }

  @Test
  void convertRewritesContentAndAParsedModel() throws Exception {
    String source = Files.readString(fixture("vehicle.sysml"));
    Conversion conversion =
        connection.convert(
            source, "sysml", ConversionOptions.defaults().withFromFormat("sysml"));
    assertFalse(conversion.content().isBlank());
    assertEquals("sysml", conversion.fromFormat());
    assertEquals("sysml", conversion.toFormat());
    assertTrue(conversion.content().contains("package"));

    Model model = connection.load(fixture("vehicle.sysml"));
    Conversion roundTrip = model.convert("sysml");
    assertFalse(roundTrip.content().isBlank());
  }

  @Test
  void aSysMLv1ModelIsMigratedNotConverted() {
    Path vehicle = v1Fixture("vehicle.xmi");
    ServiceException byExtension =
        assertThrows(ServiceException.class, () -> connection.convertFile(vehicle, "sysml"));
    assertEquals(StatusCode.INVALID_ARGUMENT, byExtension.status());
    assertTrue(byExtension.serviceMessage().contains(vehicle.toString()));
    assertTrue(
        byExtension.serviceMessage().contains("is a SysML v1 model, which is migrated, not converted"));
    assertTrue(byExtension.serviceMessage().contains("call migrateFile"));

    ConversionOptions asMdzip = ConversionOptions.defaults().withFromFormat("mdzip");
    ServiceException byFormat =
        assertThrows(
            ServiceException.class, () -> connection.convert("<xmi/>", "sysml", asMdzip));
    assertEquals(StatusCode.INVALID_ARGUMENT, byFormat.status());
    assertTrue(byFormat.serviceMessage().contains("the source is a SysML v1 model"));
    assertTrue(byFormat.serviceMessage().contains("call migrate"));

    // A form is read as the service reads it, in any case and padding.
    ConversionOptions asMDZIP = ConversionOptions.defaults().withFromFormat(" MDZIP ");
    ServiceException bySpelling =
        assertThrows(
            ServiceException.class, () -> connection.convert("<xmi/>", "sysml", asMDZIP));
    assertEquals(StatusCode.INVALID_ARGUMENT, bySpelling.status());
    assertTrue(bySpelling.serviceMessage().contains("the source is a SysML v1 model"));

    // Named by its format, the client lets the service judge; it refuses the same way.
    ConversionOptions asXmi = ConversionOptions.defaults().withFromFormat("xmi");
    ServiceException byService =
        assertThrows(
            ServiceException.class, () -> connection.convertFile(vehicle, "sysml", asXmi));
    assertEquals(StatusCode.INVALID_ARGUMENT, byService.status());
    assertTrue(byService.serviceMessage().contains("migrated, not converted"));
  }

  @Test
  void migrateAccountsForEveryElement() throws Exception {
    Path vehicle = v1Fixture("vehicle.xmi");
    Migration byFile = connection.migrateFile(vehicle, "sysml");
    assertTrue(byFile.content().contains("part def Vehicle"));
    assertEquals("xmi", byFile.fromFormat());
    assertEquals("sysml", byFile.toFormat());
    assertFalse(byFile.experimentalNotice().isBlank());
    MigrationReport summary = byFile.report();
    assertTrue(summary.summary().startsWith("migrated "));
    assertTrue(summary.mapped() > 0);
    assertTrue(summary.entries().isEmpty(), "the report was not asked for");
    assertTrue(summary.text().isEmpty());
    assertTrue(byFile.results().isEmpty());

    Migration byContent =
        connection.migrate(
            Files.readAllBytes(vehicle),
            "sysml",
            MigrationOptions.defaults().withFromFormat("xmi").withReport(true).withResults(true));
    assertEquals(byFile.content(), byContent.content());
    MigrationReport report = byContent.report();
    assertFalse(report.entries().isEmpty(), "the report was asked for");
    assertEquals(report.mapped(), report.byVerdict("mapped").size());
    assertEquals(report.approximated(), report.byVerdict("approximated").size());
    assertEquals(report.unmapped(), report.byVerdict("unmapped").size());
    assertEquals(report.skipped(), report.byVerdict("skipped").size());
    assertTrue(report.text().startsWith("# SysML v1 to v2 migration report"));
    assertTrue(report.text().contains(report.summary()));
    assertFalse(byContent.results().isEmpty());

    Migration asTurtle = connection.migrateFile(vehicle, "ttl");
    assertEquals("ttl", asTurtle.toFormat());
    assertTrue(asTurtle.content().contains("@prefix"));

    Migration laidOut =
        connection.migrateFile(
            v1Fixture("layout.xmi"),
            "sysml",
            MigrationOptions.defaults().withLayoutFile(v1Fixture("layout.layout.xml")));
    assertTrue(laidOut.report().summary().contains("laid out"));
    Migration inlineLayout =
        connection.migrateFile(
            v1Fixture("layout.xmi"),
            "sysml",
            MigrationOptions.defaults()
                .withLayoutContent(Files.readString(v1Fixture("layout.layout.xml"))));
    assertEquals(laidOut.content(), inlineLayout.content());
  }

  @Test
  void migrateRefusesWhatIsNotAv1Model() {
    Path v2 = fixture("vehicle.sysml");
    ServiceException byExtension =
        assertThrows(ServiceException.class, () -> connection.migrateFile(v2, "sysml"));
    assertEquals(StatusCode.INVALID_ARGUMENT, byExtension.status());
    assertTrue(byExtension.serviceMessage().contains("converted, not migrated"));

    MigrationOptions asNotation = MigrationOptions.defaults().withFromFormat("sysml");
    byte[] notation = "package P;".getBytes();
    ServiceException byFormat =
        assertThrows(
            ServiceException.class, () -> connection.migrate(notation, "sysml", asNotation));
    assertEquals(StatusCode.INVALID_ARGUMENT, byFormat.status());
    assertTrue(byFormat.serviceMessage().contains("call convert"));

    byte[] xmi = "<xmi/>".getBytes();
    MigrationOptions unspecified = MigrationOptions.defaults();
    ServiceException unnamed =
        assertThrows(ServiceException.class, () -> connection.migrate(xmi, "sysml", unspecified));
    assertEquals(StatusCode.INVALID_ARGUMENT, unnamed.status());

    MigrationOptions asXmi = MigrationOptions.defaults().withFromFormat("xmi");
    MigrationException unreadable =
        assertThrows(MigrationException.class, () -> connection.migrate(xmi, "sysml", asXmi));
    assertFalse(unreadable.getMessage().isEmpty());

    Path nonexistent = v1Fixture("nonexistent.xmi");
    ServiceException missing =
        assertThrows(ServiceException.class, () -> connection.migrateFile(nonexistent, "sysml"));
    assertEquals(StatusCode.NOT_FOUND, missing.status());
  }

  @Test
  void convertOfUnreadableNotationIsAModelFailure() throws Exception {
    String source = Files.readString(fixture("syntax_error.sysml"));
    ConversionOptions convertOptions = ConversionOptions.defaults().withFromFormat("sysml");
    ConversionException failed =
        assertThrows(
            ConversionException.class, () -> connection.convert(source, "sysml", convertOptions));
    assertFalse(failed.diagnostics().isEmpty());
    Model broken = connection.parse(source);
    ConversionException fromModel =
        assertThrows(ConversionException.class, () -> broken.convert("sysml"));
    assertFalse(fromModel.diagnostics().isEmpty());
  }

  @Test
  void applyEditsRewritesAValueAndAnswersTheText() {
    Model model = connection.load(fixture("editable.sysml"));
    EditResult result =
        model.applyEdits(List.of(new Edit.SetValue("Demo::SC::unitMass", "1050.0[SI::kg]")));
    assertFalse(result.content().isBlank());
    assertTrue(result.content().contains("1050.0"));
    assertEquals(1, result.applied().size());
    assertEquals("Demo::SC::unitMass", result.applied().get(0).target());
  }

  @Test
  void applyEditsRefusesAnUnknownTargetByKind() {
    Model model = connection.load(fixture("editable.sysml"));
    List<Edit> edits = List.of(new Edit.SetValue("Demo::SC::nope", "1.0"));
    EditException refused =
        assertThrows(EditException.class, () -> model.applyEdits(edits));
    assertEquals(EditFailure.UNKNOWN_TARGET, refused.failure());
    assertEquals("EDIT_FAILURE_UNKNOWN_TARGET", refused.failureName());
  }

  @Test
  void runSweepStepsThroughARangeAndReportsEachRow() {
    Model model = connection.load(fixture("sweep.sysml"));
    Sweep sweep =
        model.runSweep(
            "Sw::Sum",
            List.of(
                org.openmbee.opensysml.SweepRange.of(
                        "b", new Value.RealValue(0.0), new Value.RealValue(4.0))
                    .withStep(new Value.RealValue(2.0))),
            SweepOptions.defaults().withArguments(List.of(new Value.RealValue(1.0))));
    assertEquals(List.of("b"), sweep.parameters());
    assertEquals(3, sweep.rows().size());
    assertFalse(sweep.sampled());
    assertEquals(
        new Value.RealValue(1.0), sweep.rows().get(0).outputs().get("result"));
    assertFalse(sweep.rows().get(0).failed());
  }

  @Test
  void runSweepOfAnotherKindIsAModelFailure() {
    Model model = connection.load(fixture("sweep.sysml"));
    List<org.openmbee.opensysml.SweepRange> ranges =
        List.of(
            org.openmbee.opensysml.SweepRange.of(
                    "limit", new Value.RealValue(0.0), new Value.RealValue(4.0))
                .withStep(new Value.RealValue(2.0)));
    ModelException failed =
        assertThrows(
            ModelException.class,
            () -> model.runSweep("Sw::barge", ranges));
    assertEquals(FailureReason.WRONG_KIND, failed.failureReason());
  }

  @Test
  void runDocumentQueryAnswersTypedRows() {
    Model model = connection.load(fixture("document.sysml"));
    DocumentQueryResult result =
        model.runDocumentQuery(
            "Observatory::SubsystemTable",
            Map.of(
                "root",
                List.of(new DocumentValue.ElementRef("Observatory::telescope", ""))));
    assertEquals(List.of("name", "mass"), result.columns());
    assertEquals(4, result.rows().size());
    assertEquals(
        "Observatory::telescope::baffle|shroud *tricky*", result.rows().get(0).element().id());
    assertEquals(
        List.of(new DocumentValue.StringValue("baffle|shroud *tricky*")),
        result.rows().get(0).cells().get(0));
  }

  @Test
  void renderDocumentRendersTheNamedDocument() {
    Model model = connection.load(fixture("document.sysml"));
    RenderedDocument rendered = model.renderDocument("Observatory::MassReport");
    assertTrue(rendered.markdown().contains("# Telescope Mass Report"));
  }

  @Test
  void renderDocumentOfAnUnknownDocumentIsNotFound() {
    Model model = connection.load(fixture("document.sysml"));
    ServiceException refused =
        assertThrows(
            ServiceException.class, () -> model.renderDocument("Observatory::NoSuchDocument"));
    assertEquals(StatusCode.NOT_FOUND, refused.status());
  }

  @Test
  void renderDocumentRendersHtmlWhenAskedFor() {
    Model model = connection.load(fixture("document.sysml"));
    RenderedDocument rendered = model.renderDocument("Observatory::MassReport", DocumentForm.HTML);
    assertEquals(DocumentForm.HTML, rendered.form());
    assertTrue(rendered.html().contains("<h1"), rendered.html());
    assertTrue(rendered.html().contains("Telescope Mass Report"));
    assertEquals("", rendered.markdown());
  }

  @Test
  void renderViewReturnsTypedPortsEdgesAndOrigins() {
    Model model = connection.load(fixture("views.sysml"));
    RenderedView rendered = model.renderView("RenderViewDemo::connections");
    assertEquals("interconnection", rendered.kind());
    assertEquals(1, rendered.edges().size());
    assertFalse(rendered.edges().get(0).fromPort().isEmpty());
    assertFalse(rendered.edges().get(0).toPort().isEmpty());
    assertTrue(rendered.nodes().stream().allMatch(node -> node.origin().isPresent()));
    assertTrue(
        model
            .renderView("RenderViewDemo::connections", RenderViewPorts.FULL)
            .nodes()
            .stream()
            .flatMap(node -> node.ports().stream())
            .anyMatch(port -> port.name().equals("spare")));
  }

  @Test
  void exportGraphsAnswersTheCanonicalFormOfABehavior() {
    Model model = connection.load(fixture("behavior.sysml"));
    Graphs graphs = model.exportGraphs("Test::race");
    assertEquals(1, graphs.version());
    assertEquals("Test::race", graphs.subject());
    assertTrue(
        graphs.content().startsWith("{\"version\":1,\"subject\":\"Test::race\""),
        graphs.content());
    assertTrue(graphs.content().endsWith("}\n"), graphs.content());
    assertTrue(graphs.content().contains("\"actions\""), graphs.content());
    ServiceException missing =
        assertThrows(ServiceException.class, () -> model.exportGraphs("Test::Missing"));
    assertTrue(missing.getMessage().contains("Test::Missing"), missing.getMessage());
    ServiceException notABehavior =
        assertThrows(ServiceException.class, () -> model.exportGraphs("Test"));
    assertTrue(notABehavior.getMessage().contains("no lowered graph"), notABehavior.getMessage());
  }

  @Test
  void convertSpellsDerivedIdsInTheFormAskedFor() {
    String source = "package P { part def A; part a : A; }\n";
    ConversionOptions options = ConversionOptions.defaults().withFromFormat("sysml");
    Conversion qualified = connection.convert(source, "api-json", options);
    Conversion uuid =
        connection.convert(
            source, "api-json", options.withIdForm(ConversionOptions.ID_FORM_UUID));
    assertTrue(qualified.content().contains("\"P__a\""), qualified.content());
    assertFalse(uuid.content().contains("\"P__a\""), uuid.content());
    assertEquals(
        qualified.content(),
        connection
            .convert(source, "api-json", options.withIdForm(ConversionOptions.ID_FORM_QUALIFIED))
            .content());
    ConversionOptions shortIds = options.withIdForm("short");
    ServiceException refused =
        assertThrows(
            ServiceException.class, () -> connection.convert(source, "api-json", shortIds));
    assertEquals(StatusCode.INVALID_ARGUMENT, refused.status());
  }

  @Test
  void aConversionAndAnEditResultWriteTheirContent(@org.junit.jupiter.api.io.TempDir Path dir)
      throws Exception {
    Model model = connection.load(fixture("vehicle.sysml"));
    Path written = model.convert("sysml").write(dir.resolve("vehicle.sysml"));
    assertEquals(model.convert("sysml").content(), Files.readString(written));
    EditResult result =
        connection
            .load(fixture("editable.sysml"))
            .edit()
            .setValue("Demo::SC::unitMass", "1050.0[SI::kg]")
            .apply();
    Path saved = result.save(dir.resolve("edited.sysml"));
    assertEquals(result.content(), Files.readString(saved));
    assertTrue(connection.load(saved).ok());
  }

  @Test
  void anEmptiedDocumentOfSeveralIsNotSavedAsOneFile(@org.junit.jupiter.api.io.TempDir Path dir) throws Exception {
    Model model =
        connection.parseSources(
            List.of(
                SourceDocument.inline("a.sysml", "package A { part def P; }"),
                SourceDocument.inline("b.sysml", "package B;")));
    EditResult result =
        model
            .edit()
            .delete("B")
            .apply(EditOptions.defaults().withAcceptDocuments(true).withDocument("b.sysml"));
    assertTrue(result.severalDocuments());
    assertEquals("", result.content());
    Path target = Files.writeString(dir.resolve("kept.sysml"), "kept");
    assertThrows(IllegalStateException.class, () -> result.save(target));
    assertEquals("kept", Files.readString(target));
  }

  @Test
  void anAdoptedModelKnowsWhetherAnEmptiedEditHasSeveralDocuments(
      @org.junit.jupiter.api.io.TempDir Path dir) throws Exception {
    Model several =
        connection.model(
            connection
                .parseSources(
                    List.of(
                        SourceDocument.inline("a.sysml", "package A { part def P; }"),
                        SourceDocument.inline("b.sysml", "package B;")))
                .hash());
    EditResult refused =
        several
            .edit()
            .delete("B")
            .apply(EditOptions.defaults().withDocument("b.sysml"));
    assertTrue(refused.severalDocuments());
    Path target = Files.writeString(dir.resolve("kept.sysml"), "kept");
    assertThrows(IllegalStateException.class, () -> refused.save(target));
    assertEquals("kept", Files.readString(target));

    Model sole = connection.model(connection.parse("part def Gone;").hash());
    EditResult emptied = sole.edit().delete("Gone").apply();
    assertFalse(emptied.severalDocuments());
    assertEquals(emptied.content(), Files.readString(emptied.save(target)));
  }

  @Test
  void anEditorAppliesActionBodyStatementsOnce() {
    Model model = connection.load(fixture("apply_edits_action_body.sysml"));
    Editor editor =
        model
            .edit()
            .addIf(
                "A",
                "x < 3",
                new Editor.Body().addAssign("x", "x + 1"),
                new Editor.Body().addTerminate());
    EditResult result = editor.apply();
    assertTrue(result.content().contains("if x < 3"), result.content());
    assertTrue(result.content().contains("assign x := x + 1"), result.content());
    assertTrue(result.content().contains("terminate"), result.content());
    assertTrue(editor.applied());
    assertThrows(IllegalStateException.class, editor::apply);
    assertThrows(IllegalStateException.class, () -> editor.addTerminate("A"));
  }

  @Test
  void anEmptyEditorIsRefusedByTheService() {
    Model model = connection.load(fixture("editable.sysml"));
    Editor empty = model.edit();
    EditException refused = assertThrows(EditException.class, empty::apply);
    assertEquals(EditFailure.NO_OPERATIONS, refused.failure());
  }

  @Test
  void lookupsFindByShortNameOrIdAndNameWhatIsMissing() {
    Model model = connection.parse(VEHICLE);
    assertEquals("Demo::Vehicle", model.find("Vehicle").orElseThrow().id());
    assertEquals("Demo::Vehicle", model.find("Demo::Vehicle").orElseThrow().id());
    assertEquals("Demo::Vehicle::engine", model.lookup("engine").id());
    assertTrue(model.find("Nope").isEmpty());
    assertTrue(model.get("Vehicle").isEmpty());
    assertEquals("Demo::Engine", model.get("Demo::Engine").orElseThrow().id());
    assertTrue(model.contains("Engine"));
    assertFalse(model.contains("Nope"));
    SymbolNotFoundException missing =
        assertThrows(SymbolNotFoundException.class, () -> model.lookup("Vehicel"));
    assertEquals("Vehicel", missing.name());
    assertTrue(missing.suggestions().contains("Vehicle"), missing.suggestions().toString());
  }

  @Test
  void aModelNamesItsDocumentsAndRefusesToPassWithErrors() {
    Path broken = fixture("syntax_error.sysml");
    Model model = connection.load(broken);
    assertEquals(List.of(broken.toString()), model.documents());
    assertFalse(model.ok());
    assertFalse(model.errors().isEmpty());
    ModelException refused = assertThrows(ModelException.class, model::requireNoErrors);
    assertTrue(refused.getMessage().startsWith(broken.toString()), refused.getMessage());
    Model clean = connection.load(fixture("vehicle.sysml"));
    assertEquals(clean, clean.requireNoErrors());
    assertEquals(
        List.of("a.sysml", "b.sysml"),
        connection
            .parseSources(
                List.of(
                    SourceDocument.inline("a.sysml", "package A;"),
                    SourceDocument.inline("b.sysml", "package B;")))
            .documents());
  }

  @Test
  void notFoundRefusalsAreTyped() {
    Model evicted = new Model(connection, "no-such-hash", List.of(), List.of());
    ModelNotFoundException model =
        assertThrows(ModelNotFoundException.class, () -> evicted.eval("1 + 1"));
    assertEquals(StatusCode.NOT_FOUND, model.status());
    assertThrows(ModelNotFoundException.class, () -> evicted.convert("sysml"));
    Editor edit = evicted.edit().addPart("A", "b");
    assertThrows(ModelNotFoundException.class, edit::apply);
    Path noSuchModel = Path.of("/no/such/model.sysml");
    ModelFileNotFoundException file =
        assertThrows(ModelFileNotFoundException.class, () -> connection.load(noSuchModel));
    assertEquals(StatusCode.NOT_FOUND, file.status());
  }

  @Test
  void aConnectionRefusesAServiceLackingARequiredCapability() {
    ConnectionOptions options =
        ServiceBinary.options()
            .requireCapabilities(Capabilities.QUERY, "no_such_capability")
            .build();
    CapabilityException refused =
        assertThrows(CapabilityException.class, () -> Connection.open(options));
    assertEquals("no_such_capability", refused.capability());
    try (Connection checked =
        Connection.open(
            ServiceBinary.options().requireCapabilities(List.of(Capabilities.QUERY)).build())) {
      assertTrue(checked.capabilities().has(Capabilities.QUERY));
    }
  }

  @Test
  void aConnectionRefusesAServiceThatIsNotTheReleaseAskedFor() {
    ConnectionOptions options = ServiceBinary.options().downloadVersion("v0.0.1").build();
    StaleServiceException stale =
        assertThrows(StaleServiceException.class, () -> Connection.open(options));
    assertTrue(stale.reason().contains("v0.0.1"), stale.reason());
    assertEquals(connection.capabilities().serviceVersion(), stale.serviceVersion());
  }
}
