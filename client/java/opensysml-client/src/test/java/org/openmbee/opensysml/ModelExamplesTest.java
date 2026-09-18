package org.openmbee.opensysml;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertNotEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.Set;
import java.util.concurrent.Callable;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.Future;
import java.util.concurrent.TimeUnit;
import java.util.stream.Collectors;
import org.junit.jupiter.api.AfterAll;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInstance;

/**
 * Worked examples of the API against real models: what each value kind, symbol shape and failure
 * looks like to a caller. {@link ApiIntegrationTest} covers the v1 surface once; this covers the
 * breadth of models a caller brings to it.
 */
@TestInstance(TestInstance.Lifecycle.PER_CLASS)
class ModelExamplesTest {

  /** Values that hold nothing: a declared feature with no value, and a sequence of them. */
  private static final String UNSET =
      """
      package Unset {
        private import ScalarValues::*;
        part def Engine { attribute power : Real = 120.0; }
        part def Vehicle {
          attribute d : Real;
          attribute ds : Real[2];
          attribute k : Real = 2.0;
          part engine : Engine;
        }
      }
      """;

  /** A specialization chain that redefines twice, plus every multiplicity a usage can carry. */
  private static final String INHERITANCE =
      """
      package Inherit {
        part def Base { attribute a default = 1; attribute b default = 2; }
        part def Mid :> Base { attribute :>> b = 20; attribute c = 3; }
        part def Leaf :> Mid { attribute :>> a = 100; }
        part def Holder {
          part many : Leaf[0..*];
          part exactlyTwo : Leaf[2];
          part optional : Leaf[0..1];
        }
        part holder : Holder { part :>> many : Leaf[3]; }
      }
      """;

  /** One declaration of each kind the service names, so the client's symbol kinds are exercised. */
  private static final String KINDS =
      """
      package Kinds {
        attribute def Temp :> ScalarValues::Real;
        enum def Color { red; green; blue; }
        part def Tank {
          attribute level : ScalarValues::Real default = 0.5;
          attribute colour : Color = Color::green;
          assert constraint notOverfull { level <= 1.0 }
        }
        requirement def Safe { subject t : Tank; require constraint { t.level < 1.0 } }
        calc def Total { in a : ScalarValues::Real; in b : ScalarValues::Real; return : ScalarValues::Real = a + b; }
        action def Fill { in amount : ScalarValues::Real; }
        state def Operating { state idle; state running; transition idle then running; }
        part tank : Tank { attribute :>> level = 0.9; }
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
  void everyValueKindReadsBackAsItsOwnType() {
    Model model = connection.parse("package Values { attribute a = 1; }");
    assertEquals(new Value.IntegerValue(7), model.eval("3 + 4"));
    assertEquals(new Value.RealValue(0.5), model.eval("1.0 / 2.0"));
    assertEquals(new Value.BooleanValue(true), model.eval("1 < 2"));
    assertEquals(new Value.StringValue("a b"), model.eval("\"a b\""));
    assertEquals(new Value.NullValue(), model.eval("null"));
    assertEquals(
        new Value.Sequence(
            List.of(new Value.IntegerValue(1), new Value.IntegerValue(2), new Value.IntegerValue(3))),
        model.eval("(1, 2, 3)"));
    assertEquals(new Value.Sequence(List.of()), model.eval("(1, 2, 3).?{in e; e > 5}"));
    assertEquals(new Value.NullValue(), model.eval("()"));
  }

  @Test
  void aValueOfTheWrongKindIsRefusedRatherThanConverted() {
    Model model = connection.parse("package Convert { attribute a = 1; }");
    assertEquals(2L, model.eval("2").asLong());
    assertEquals(2.0, model.eval("2").asDouble());
    Value real = model.eval("2.7");
    Value string = model.eval("\"x\"");
    Value bool = model.eval("true");
    Value nothing = model.eval("null");
    assertThrows(IllegalStateException.class, real::asLong);
    assertThrows(IllegalStateException.class, string::asDouble);
    assertThrows(IllegalStateException.class, bool::asLong);
    assertThrows(IllegalStateException.class, nothing::asString);
  }

  @Test
  void aFeatureThatHoldsNoValueIsUnsetRatherThanAbsentOrNull() {
    Model model = connection.parse(UNSET);
    assertTrue(connection.capabilities().has(Capabilities.UNSET_VALUE));
    Instance vehicle = model.instantiate("Unset::Vehicle").root();

    Instance.FeatureValue d = vehicle.featureValues().get("d");
    assertTrue(d.materialized(), "the object exists whether or not it holds a value");
    assertEquals(Optional.of(new Value.UnsetValue()), d.value());
    assertNotEquals(Optional.of(new Value.NullValue()), d.value());

    Instance.FeatureValue ds = vehicle.featureValues().get("ds");
    assertEquals(Optional.empty(), ds.value());
    assertEquals(List.of(new Value.UnsetValue(), new Value.UnsetValue()), ds.values());

    assertEquals(Optional.of(new Value.RealValue(2.0)), vehicle.featureValues().get("k").value());
    assertEquals(new Value.UnsetValue(), model.evalWithSubject("d", "Unset::Vehicle"));
    assertEquals(
        new Value.Sequence(List.of(new Value.UnsetValue(), new Value.UnsetValue())),
        model.evalWithSubject("ds", "Unset::Vehicle"));
  }

  @Test
  void aQuantityCarriesTheUnitAsWrittenAndWhatItReducesTo() {
    Model model =
        connection.parse(
            """
            package Units {
              private import SI::*;
              attribute mass = 5.0 [kg];
            }
            """);
    assertEquals(List.of(), model.parseDiagnostics());
    Quantity quantity = ((Value.QuantityValue) model.evalInContext("mass", "Units")).quantity();
    assertEquals(5.0, quantity.magnitude());
    assertEquals(Optional.of("kg"), quantity.unit());
    Quantity.UnitTerm reduction = quantity.reduction().orElseThrow();
    assertEquals(
        List.of("SI::gram"),
        reduction.factors().stream().map(Quantity.UnitFactor::unitId).toList());
    assertEquals(1000.0, reduction.scaleNumerator() / reduction.scaleDenominator());
    assertThrows(ModelException.class, () -> model.eval("1.0 [SI::kg] + 1.0 [SI::m]"));
  }

  @Test
  void anEnumerationValueNamesItsLiteralAndItsEnumeration() {
    Model model = connection.parse(KINDS);
    Value colour = model.evalInContext("colour", "Kinds::Tank");
    EnumLiteral literal = ((Value.EnumerationValue) colour).literal();
    assertEquals("Kinds::Color::green", literal.literalId());
    assertEquals("Kinds::Color", literal.enumerationId());
    assertEquals(
        List.of("Kinds::Color::red", "Kinds::Color::green", "Kinds::Color::blue"),
        model.symbol("Kinds::Color").childIds());
    assertEquals(new Value.BooleanValue(true), model.evalInContext("colour == Color::green", "Kinds::Tank"));
    assertEquals(
        Optional.of(colour), model.instantiate("Kinds::tank").root().featureValues().get("colour").value());
  }

  @Test
  void aRedefinedFeatureReadsTheRedefinitionAndKeepsWhatItInherits() {
    Model model = connection.parse(INHERITANCE);
    assertEquals(List.of(), model.parseDiagnostics());
    assertEquals(new Value.IntegerValue(100), model.evalInContext("a", "Inherit::Leaf"));
    assertEquals(new Value.IntegerValue(20), model.evalInContext("b", "Inherit::Leaf"));
    assertEquals(new Value.IntegerValue(3), model.evalInContext("c", "Inherit::Leaf"));

    Symbol leaf = model.symbol("Inherit::Leaf");
    assertEquals(
        Map.of("a", new Value.IntegerValue(100), "b", new Value.IntegerValue(20), "c", new Value.IntegerValue(3)),
        leaf.attributes().stream()
            .collect(Collectors.toMap(Symbol.Attribute::name, attribute -> attribute.value().orElseThrow())));
    assertEquals(
        List.of(new Symbol.Specialization("specializes", "Mid", Optional.of("Inherit::Mid"), Optional.of("partDef"))),
        leaf.specializations());
  }

  @Test
  void multiplicityIsReportedAsWrittenAndAsRedefined() {
    Model model = connection.parse(INHERITANCE);
    assertEquals(
        Optional.of(new Symbol.Multiplicity(Optional.of("0"), Optional.of("*"))),
        model.symbol("Inherit::Holder::many").multiplicity());
    assertEquals(
        Optional.of(new Symbol.Multiplicity(Optional.of("2"), Optional.of("2"))),
        model.symbol("Inherit::Holder::exactlyTwo").multiplicity());
    assertEquals(
        Optional.of(new Symbol.Multiplicity(Optional.of("0"), Optional.of("1"))),
        model.symbol("Inherit::Holder::optional").multiplicity());
    assertEquals(
        Optional.of(new Symbol.Multiplicity(Optional.of("3"), Optional.of("3"))),
        model.symbol("Inherit::holder::many").multiplicity());
    assertEquals(Optional.empty(), model.symbol("Inherit::Leaf").multiplicity());
  }

  @Test
  void aMultipliedPartMaterializesOneObjectPerElement() {
    Model model = connection.parse(INHERITANCE);
    Instantiation instantiation = model.instantiate("Inherit::holder");
    Instance.FeatureValue many = instantiation.root().featureValues().get("many");
    assertEquals(Optional.empty(), many.value(), "a multiplied feature answers values, not a value");
    assertEquals(3, many.values().size());
    for (Value element : many.values()) {
      Instance object = instantiation.resolve((Value.InstanceReference) element).orElseThrow();
      assertEquals("Inherit::Leaf", object.typeSymbolId());
      assertEquals(Optional.of(new Value.IntegerValue(100)), object.featureValues().get("a").value());
    }
    assertEquals(2, instantiation.root().featureValues().get("exactlyTwo").values().size());
  }

  @Test
  void everyDeclarationKindIsNamedAndReachableAsASymbol() {
    Model model = connection.parse(KINDS);
    Map<String, String> kinds = new java.util.LinkedHashMap<>();
    for (String id :
        List.of(
            "Kinds::Temp",
            "Kinds::Color",
            "Kinds::Tank",
            "Kinds::Tank::notOverfull",
            "Kinds::Safe",
            "Kinds::Total",
            "Kinds::Fill",
            "Kinds::Operating",
            "Kinds::Operating::idle",
            "Kinds::tank")) {
      kinds.put(id, model.symbol(id).kind());
    }
    assertEquals(
        Map.ofEntries(
            Map.entry("Kinds::Temp", "attributeDef"),
            Map.entry("Kinds::Color", "enumDef"),
            Map.entry("Kinds::Tank", "partDef"),
            Map.entry("Kinds::Tank::notOverfull", "constraintUsage"),
            Map.entry("Kinds::Safe", "requirementDef"),
            Map.entry("Kinds::Total", "calcDef"),
            Map.entry("Kinds::Fill", "actionDef"),
            Map.entry("Kinds::Operating", "stateDef"),
            Map.entry("Kinds::Operating::idle", "stateUsage"),
            Map.entry("Kinds::tank", "partUsage")),
        kinds);
  }

  @Test
  void aUsageReportsWhatItsTypeResolvedTo() {
    Model model = connection.parse(KINDS);
    Symbol.TypeFacts facts = model.symbol("Kinds::tank").typeFacts().orElseThrow();
    assertEquals(Optional.of("Tank"), facts.declared());
    assertEquals(Optional.of("Kinds::Tank"), facts.resolvedId());
    assertEquals(Optional.of("partDef"), facts.resolvedKind());
    Symbol.TypeFacts primitive = model.symbol("Kinds::Temp").typeFacts().orElseThrow();
    assertEquals(Optional.of("Real"), primitive.primitive());
  }

  @Test
  void aConstraintAndACalculationEvaluateAgainstAnObject() {
    Model model = connection.parse(KINDS);
    assertEquals(new Value.BooleanValue(true), model.evalWithSubject("level <= 1.0", "Kinds::tank"));
    assertEquals(new Value.RealValue(0.9), model.evalWithSubject("level", "Kinds::tank"));
    assertEquals(new Value.RealValue(0.5), model.evalInContext("level", "Kinds::Tank"));
    assertEquals(new Value.RealValue(3.0), model.evalInContext("Total(1.0, 2.0)", "Kinds"));
  }

  @Test
  void aSymbolFromTheStandardLibraryReportsHowManyAttributesWereWithheld() {
    Model model = connection.parse("package Library { private import ScalarValues::*; }");
    Symbol real = model.symbol("ScalarValues::Real");
    assertEquals("attributeDef", real.kind());
    assertTrue(
        real.withheldLibraryAttributes() > 0,
        "a library symbol reports the attributes it does not carry");
    assertEquals(List.of(), real.attributes());
  }

  @Test
  void aDiagnosticLocatesItselfInTheFileTheServiceRead() throws Exception {
    Path file = Files.createTempFile("opensysml", ".sysml");
    try {
      Files.writeString(file, "package Broken { part x : Nope; }\n");
      Model model = connection.load(file);
      Diagnostic diagnostic = model.parseDiagnostics().get(0);
      assertEquals(Diagnostic.Severity.ERROR, diagnostic.severity());
      Diagnostic.Span span = diagnostic.span().orElseThrow();
      assertEquals(file.toString(), span.file());
      assertEquals(1, span.startLine());
      assertEquals(model.parseDiagnostics(), model.diagnostics(), "the same model answers the same findings");
    } finally {
      Files.deleteIfExists(file);
    }
  }

  @Test
  void anExpressionThatNamesNothingIsAModelFailureCarryingItsDiagnostics() {
    Model model = connection.parse(KINDS);
    ModelException failed = assertThrows(ModelException.class, () -> model.eval("nosuchname + 1"));
    assertFalse(failed.getMessage().isBlank());
    assertThrows(ModelException.class, () -> model.eval(""));
    assertThrows(ModelException.class, () -> model.symbol("Kinds::Missing"));
    assertEquals(Optional.empty(), model.findSymbol("Kinds::Missing"));
    assertThrows(NullPointerException.class, () -> model.eval(null));
    assertThrows(NullPointerException.class, () -> connection.parse(KINDS, null));
    assertThrows(IllegalArgumentException.class, () -> connection.model("  "));
  }

  @Test
  void aJsonBodyAnswersWhatAProtobufBodyAnswersForEveryShapeOfValue() {
    try (Connection json = Connection.open(ServiceBinary.options().encoding(Encoding.JSON).build())) {
      Model viaProtobuf = connection.parse(KINDS);
      Model viaJson = json.parse(KINDS);
      assertEquals(viaProtobuf.hash(), viaJson.hash());
      assertEquals(viaProtobuf.parseDiagnostics(), viaJson.parseDiagnostics());
      for (String id : List.of("Kinds::Tank", "Kinds::tank", "Kinds::Color", "Kinds::Total")) {
        assertEquals(viaProtobuf.symbol(id), viaJson.symbol(id), id);
      }
      for (String expression : List.of("2 + 2", "1.0 / 3.0", "9223372036854775807", "(1, 2)", "null", "\"é\\t\"")) {
        assertEquals(viaProtobuf.eval(expression), viaJson.eval(expression), expression);
      }
      assertEquals(
          viaProtobuf.evalInContext("colour", "Kinds::Tank"), viaJson.evalInContext("colour", "Kinds::Tank"));
      assertEquals(
          viaProtobuf.root().orElseThrow(), viaJson.root().orElseThrow());

      // Object ids are the service's own numbering, so the graphs are compared by what they hold.
      Instantiation protobufTank = viaProtobuf.instantiate("Kinds::tank");
      Instantiation jsonTank = viaJson.instantiate("Kinds::tank");
      assertEquals(featureNames(protobufTank), featureNames(jsonTank));
      assertEquals(
          protobufTank.root().featureValues().get("level").value(),
          jsonTank.root().featureValues().get("level").value());
      assertEquals(
          protobufTank.reachable().stream().map(Instance::typeSymbolId).toList(),
          jsonTank.reachable().stream().map(Instance::typeSymbolId).toList());
    }
  }

  @Test
  void oneConnectionServesManyThreadsAtOnce() throws Exception {
    int calls = 32;
    ExecutorService pool = Executors.newFixedThreadPool(8);
    try {
      List<Callable<Value>> tasks = new ArrayList<>();
      for (int i = 0; i < calls; i++) {
        int n = i;
        tasks.add(() -> connection.parse("package Threaded" + n + " { part def A; }").eval(n + " + 1"));
      }
      List<Long> answered = new ArrayList<>();
      for (Future<Value> future : pool.invokeAll(tasks)) {
        answered.add(future.get(60, TimeUnit.SECONDS).asLong());
      }
      assertEquals(
          java.util.stream.LongStream.range(1, calls + 1).boxed().collect(Collectors.toSet()),
          Set.copyOf(answered));
    } finally {
      pool.shutdownNow();
    }
  }

  @Test
  void aModelTheSizeOfARealOneParsesAndAnswersLookups() {
    StringBuilder source = new StringBuilder("package Large {\n");
    for (int i = 0; i < 2000; i++) {
      source.append("  part def P").append(i).append(" { attribute a = ").append(i).append("; }\n");
    }
    source.append("}\n");
    Model model = connection.parse(source.toString());
    assertEquals(List.of(), model.parseDiagnostics());
    assertEquals(2000, model.symbol("Large").childIds().size());
    assertEquals(new Value.IntegerValue(1999), model.evalInContext("a", "Large::P1999"));
  }

  @Test
  void unicodeNamesAndStringsSurviveTheRoundTrip() {
    Model model =
        connection.parse(
            """
            package Unicode {
              part def 'Moteur électrique' { attribute puissance = "150 chevaux — ☃"; }
            }
            """);
    assertEquals(List.of(), model.parseDiagnostics());
    Symbol motor = model.symbol("Unicode::Moteur électrique");
    assertEquals("Moteur électrique", motor.name());
    assertEquals(
        new Value.StringValue("150 chevaux — ☃"),
        model.evalInContext("puissance", motor.id()));
    assertEquals(new Value.StringValue("héllo → ☃"), model.eval("\"héllo → ☃\""));
  }

  private static Set<String> featureNames(Instantiation instantiation) {
    return instantiation.root().featureValues().keySet();
  }
}
