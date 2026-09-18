package org.openmbee.opensysml;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.OptionalDouble;
import org.junit.jupiter.api.Test;

/** The public result types of execution, verification, analysis and query: immutable and decided. */
class ResultTypesTest {

  private static Verdict verdict(boolean holds, String error) {
    return new Verdict(
        Verdict.KIND_CONSTRAINT,
        Optional.of("Demo::Vehicle::massLight"),
        "massLight",
        holds,
        holds ? Optional.empty() : Optional.of("mass < 100.0"),
        Optional.empty(),
        Optional.empty(),
        error.isEmpty() ? Optional.empty() : Optional.of(error),
        error.isEmpty() ? FailureReason.UNSPECIFIED : FailureReason.EVALUATION,
        Optional.empty(),
        Optional.empty(),
        Standing.none());
  }

  @Test
  void aFalseVerdictIsDecidedAndAnErroredOneIsNot() {
    Verdict violated = verdict(false, "");
    assertTrue(violated.decided());
    assertTrue(violated.violated());
    assertFalse(violated.holds());

    Verdict undecided = verdict(false, "mass is unbound");
    assertFalse(undecided.decided());
    assertFalse(undecided.violated());
    assertEquals(FailureReason.EVALUATION, undecided.failureReason());

    Verdict holding = verdict(true, "");
    assertTrue(holding.decided());
    assertFalse(holding.violated());
  }

  @Test
  void aSatisfactionSortsItsVerdictsByWhatTheySay() {
    Verdict holding = verdict(true, "");
    Verdict violated = verdict(false, "");
    Verdict undecided = verdict(false, "unbound");
    Satisfaction satisfaction =
        new Satisfaction(List.of(holding, violated, undecided), List.of(), List.of(), List.of());
    assertFalse(satisfaction.holds());
    assertEquals(List.of(violated), satisfaction.violated());
    assertEquals(List.of(undecided), satisfaction.undecided());
    assertTrue(new Satisfaction(List.of(holding), List.of(), List.of(), List.of()).holds());
    assertTrue(new Satisfaction(List.of(), List.of(), List.of(), List.of()).holds());
  }

  @Test
  void aSatisfactionFindsTheVerificationCasesOfARequirement() {
    VerificationVerdict pass =
        new VerificationVerdict(
            "Demo::checkZero", VerificationVerdict.PASS, Optional.empty(), false, Optional.of("Demo::zeroed"));
    VerificationVerdict fail =
        new VerificationVerdict(
            "Demo::checkBound",
            VerificationVerdict.FAIL,
            Optional.of("bound exceeded"),
            false,
            Optional.of("Demo::bounded"));
    Satisfaction satisfaction =
        new Satisfaction(List.of(), List.of(pass, fail), List.of(), List.of());
    assertEquals(List.of(fail), satisfaction.verificationsOf("Demo::bounded"));
    assertEquals(List.of(), satisfaction.verificationsOf("Demo::other"));
    assertTrue(pass.passed());
    assertFalse(fail.passed());
  }

  @Test
  void resultsCopyTheCollectionsTheyAreGivenAndRefuseChanges() {
    List<Verdict> verdicts = new ArrayList<>(List.of(verdict(true, "")));
    Map<String, Value> outputs = new LinkedHashMap<>();
    outputs.put("total", new Value.RealValue(12.0));
    Analysis analysis =
        new Analysis(outputs, verdicts, List.of(), List.of(), List.of(), List.of(), Standing.none());
    verdicts.clear();
    outputs.put("later", new Value.RealValue(1.0));
    assertEquals(1, analysis.verdicts().size());
    assertEquals(List.of("total"), List.copyOf(analysis.outputs().keySet()));
    Value added = new Value.RealValue(2.0);
    assertThrows(UnsupportedOperationException.class, () -> analysis.outputs().put("x", added));
    Verdict extra = verdict(false, "");
    assertThrows(UnsupportedOperationException.class, () -> analysis.verdicts().add(extra));

    Map<String, Value> context = new LinkedHashMap<>(Map.of("n", new Value.IntegerValue(1)));
    StateRun run =
        new StateRun(new ArrayList<>(List.of("init", "done")), context, OptionalDouble.empty(), List.of());
    context.clear();
    assertEquals(Optional.of("done"), run.finalState());
    assertEquals(1, run.finalContext().size());
    assertEquals(
        Optional.empty(),
        new StateRun(List.of(), Map.of(), OptionalDouble.empty(), List.of()).finalState());
  }

  @Test
  void anOutputOrderIsTheOneTheServiceReported() {
    Map<String, Value> outputs = new LinkedHashMap<>();
    outputs.put("z", new Value.IntegerValue(1));
    outputs.put("a", new Value.IntegerValue(2));
    Calculation calculation =
        new Calculation(Optional.empty(), outputs, List.of(), Standing.none());
    assertEquals(List.of("z", "a"), List.copyOf(calculation.outputs().keySet()));
  }

  @Test
  void aCalculationsValueIsItsResultOrItsOnlyOutput() {
    Value five = new Value.IntegerValue(5);
    assertEquals(
        Optional.of(five),
        new Calculation(Optional.of(five), Map.of(), List.of(), Standing.none()).value());
    assertEquals(
        Optional.of(five),
        new Calculation(Optional.empty(), Map.of("out", five), List.of(), Standing.none()).value());
    Map<String, Value> two = new LinkedHashMap<>();
    two.put("a", five);
    two.put("b", five);
    assertEquals(
        Optional.empty(), new Calculation(Optional.empty(), two, List.of(), Standing.none()).value());
  }

  @Test
  void anAnalysisNamesItsObjectiveItsSelectedAlternativeAndTheObjectsItRefersTo() {
    Instance engine = new Instance(7, "Trade::b", Map.of());
    Verdict objective =
        new Verdict(
            Verdict.KIND_OBJECTIVE,
            Optional.of("Trade::lightest::objective"),
            "tradeStudyObjective",
            true,
            Optional.empty(),
            Optional.of(7L),
            Optional.of("Trade::b"),
            Optional.empty(),
            FailureReason.UNSPECIFIED,
            Optional.empty(),
            Optional.empty(),
            Standing.none());
    CaseEvaluation loser =
        new CaseEvaluation(
            "Trade::lightest::evaluationFunction",
            List.of(new Value.InstanceReference(3)),
            Optional.of(new Value.RealValue(30.0)),
            Optional.empty(),
            false,
            false);
    CaseEvaluation winner =
        new CaseEvaluation(
            "Trade::lightest::evaluationFunction",
            List.of(new Value.InstanceReference(7)),
            Optional.of(new Value.RealValue(10.0)),
            Optional.empty(),
            true,
            false);
    Analysis analysis =
        new Analysis(
            Map.of("selectedAlternative", new Value.InstanceReference(7)),
            List.of(objective),
            List.of(),
            List.of(loser, winner),
            List.of(engine),
            List.of(),
            new Standing("run", "observed", List.of()));
    assertTrue(analysis.holds());
    assertEquals(Optional.of(objective), analysis.objective());
    assertEquals(Optional.of(winner), analysis.selected());
    assertEquals(Optional.of(engine), analysis.resolve(new Value.InstanceReference(7)));
    assertEquals(Optional.empty(), analysis.resolve(new Value.InstanceReference(3)));
    assertEquals(Optional.of(engine), analysis.instance(7));
    assertTrue(analysis.standing().reported());
  }

  @Test
  void anAnalysisExceptionKeepsWhatTheRunLeft() {
    Analysis partial =
        new Analysis(
            Map.of(), List.of(verdict(false, "division by zero")), List.of(), List.of(), List.of(), List.of(), Standing.none());
    AnalysisException e =
        new AnalysisException("division by zero", FailureReason.EVALUATION, List.of(), partial);
    assertEquals(Optional.of(partial), e.partial());
    assertEquals(FailureReason.EVALUATION, e.failureReason());
    assertEquals("division by zero", e.getMessage());
  }

  @Test
  void anExplorationReportsHowItEnded() {
    Outcome one =
        new Outcome(
            Map.of("x", new Value.IntegerValue(1)),
            Optional.empty(),
            List.of(),
            Optional.empty(),
            2,
            List.of("first of a, b, c: a"),
            List.of());
    Outcome failed =
        new Outcome(
            Map.of(), Optional.empty(), List.of(), Optional.of("deadlock"), 1, List.of(), List.of());
    assertTrue(one.completed());
    assertFalse(failed.completed());
    assertEquals(
        "complete (6 runs)",
        new Exploration(List.of(one), true, 6, List.of(), 1024, 64).status());
    assertEquals(
        "incomplete: runs budget 100 hit after 100 runs",
        new Exploration(List.of(one, failed), false, 100, List.of("runs"), 100, 64).status());
    assertEquals(
        "incomplete: runs budget 4 and depth budget 2 hit after 4 runs",
        new Exploration(List.of(), false, 4, List.of("runs", "depth"), 4, 2).status());
  }

  @Test
  void executionOptionsKnowWhetherTheyExplore() {
    assertFalse(ExecutionOptions.defaults().explores());
    assertFalse(ExecutionOptions.defaults().withSchedule("declared").explores());
    assertTrue(ExecutionOptions.defaults().withSchedule("explore").explores());
    assertTrue(ExecutionOptions.defaults().withSchedule("explore:runs=10,depth=4").explores());
    ExecutionOptions options =
        ExecutionOptions.defaults().withSchedule("seed:7").withPerformer("Demo::sedan");
    assertEquals(Optional.of("seed:7"), options.schedule());
    assertEquals(Optional.of("Demo::sedan"), options.performer());
    assertEquals(Optional.empty(), ExecutionOptions.defaults().performer());
  }

  @Test
  void analysisOptionsAccumulateAndCopyTheirArguments() {
    List<Value> positional = new ArrayList<>(List.of(new Value.RealValue(10.0)));
    Map<String, Value> named = new LinkedHashMap<>(Map.of("limit", new Value.RealValue(50.0)));
    AnalysisOptions options =
        AnalysisOptions.defaults()
            .withSubject("An::barge")
            .withArguments(positional)
            .withNamedArguments(named)
            .withSchedule("explore");
    positional.clear();
    named.clear();
    assertEquals(Optional.of("An::barge"), options.subject());
    assertEquals(List.of(new Value.RealValue(10.0)), options.arguments());
    assertEquals(Map.of("limit", new Value.RealValue(50.0)), options.namedArguments());
    assertTrue(options.explores());
    assertFalse(AnalysisOptions.defaults().explores());
    Value extra = new Value.RealValue(1.0);
    assertThrows(UnsupportedOperationException.class, () -> options.arguments().add(extra));
  }

  @Test
  void aQueryIsBuiltUpAndItsConditionsNegate() {
    Condition.Comparison parts = Condition.equal("@type", List.of("PartUsage", "PartDefinition"));
    Condition.Comparison heavy = Condition.greater("mass", "1000");
    Query query =
        Query.all()
            .withScope(List.of("Demo"))
            .withSelect(List.of("name", "qualifiedName"))
            .where(Condition.all(List.of(parts, heavy)));
    assertEquals(List.of("Demo"), query.scope());
    assertEquals(List.of("name", "qualifiedName"), query.select());
    assertEquals(Optional.of(Condition.all(List.of(parts, heavy))), query.where());
    assertEquals(Optional.empty(), Query.all().where());

    assertEquals(
        new Condition.Comparison(
            "@type", Condition.Comparison.Operator.EQUAL, List.of("PartUsage", "PartDefinition"), true),
        parts.negated());
    assertEquals(parts, parts.negated().negated());
    assertEquals(
        Condition.any(List.of(parts.negated(), heavy.negated())),
        Condition.all(List.of(parts, heavy)).negated());
    assertEquals(
        Condition.all(List.of(parts.negated(), heavy.negated())),
        Condition.any(List.of(parts, heavy)).negated());
    assertEquals(Condition.Comparison.Operator.LESS, Condition.less("mass", "10").operator());
  }

  @Test
  void aQueryElementCopiesItsProperties() {
    Map<String, String> properties = new LinkedHashMap<>(Map.of("name", "sedan"));
    QueryElement element = new QueryElement("Demo::sedan", "PartUsage", properties);
    properties.clear();
    assertEquals(Map.of("name", "sedan"), element.properties());
    assertThrows(UnsupportedOperationException.class, () -> element.properties().put("a", "b"));
  }

  @Test
  void aStandingIsReportedOnlyWhenAnEngineAnswered() {
    assertFalse(Standing.none().reported());
    Standing standing =
        new Standing("explore", "observed", List.of(new Standing.Bound("runs", 1024, false)));
    assertTrue(standing.reported());
    assertEquals("explore", standing.engine());
    assertEquals(1024, standing.bounds().get(0).limit());
    assertEquals("auto", Standing.ENGINE_AUTO);
    assertEquals("all", Standing.ENGINE_ALL);
  }

  @Test
  void aVerificationFindsItsSubjectAmongItsInstances() {
    Instance sedan = new Instance(1, "Demo::sedan", Map.of());
    Verdict about =
        new Verdict(
            Verdict.KIND_CONSTRAINT,
            Optional.of("Demo::Vehicle::massPositive"),
            "massPositive",
            true,
            Optional.empty(),
            Optional.of(1L),
            Optional.of("Demo::sedan"),
            Optional.empty(),
            FailureReason.UNSPECIFIED,
            Optional.empty(),
            Optional.empty(),
            Standing.none());
    Verification verification = new Verification(about, List.of(), List.of(sedan), List.of());
    assertTrue(verification.holds());
    assertEquals(Optional.of(sedan), verification.subject());
    assertEquals(Optional.empty(), verification.instance(2));
    Verification declared =
        new Verification(verdict(true, ""), List.of(), List.of(sedan), List.of());
    assertEquals(Optional.empty(), declared.subject());
  }
}
