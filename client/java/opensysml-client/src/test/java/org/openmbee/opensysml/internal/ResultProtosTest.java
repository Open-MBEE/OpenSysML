package org.openmbee.opensysml.internal;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.OptionalDouble;
import org.junit.jupiter.api.Test;
import org.openmbee.opensysml.ActionRun;
import org.openmbee.opensysml.Analysis;
import org.openmbee.opensysml.Calculation;
import org.openmbee.opensysml.Condition;
import org.openmbee.opensysml.EngineInfo;
import org.openmbee.opensysml.Exploration;
import org.openmbee.opensysml.FailureReason;
import org.openmbee.opensysml.Query;
import org.openmbee.opensysml.QueryElement;
import org.openmbee.opensysml.Quantity;
import org.openmbee.opensysml.Standing;
import org.openmbee.opensysml.StateRun;
import org.openmbee.opensysml.Validation;
import org.openmbee.opensysml.Value;
import org.openmbee.opensysml.Verdict;
import org.openmbee.opensysml.Verification;
import org.openmbee.opensysml.VerificationVerdict;
import org.openmbee.opensysml.proto.Bound;
import org.openmbee.opensysml.proto.CalcOutput;
import org.openmbee.opensysml.proto.CaseEvaluation;
import org.openmbee.opensysml.proto.CompositeOperator;
import org.openmbee.opensysml.proto.Constraint;
import org.openmbee.opensysml.proto.Diagnostic;
import org.openmbee.opensysml.proto.EvaluateCalcResponse;
import org.openmbee.opensysml.proto.ExecuteActionResponse;
import org.openmbee.opensysml.proto.ExecuteStateResponse;
import org.openmbee.opensysml.proto.ExplorationStatus;
import org.openmbee.opensysml.proto.Instance;
import org.openmbee.opensysml.proto.Outcome;
import org.openmbee.opensysml.proto.PrimitiveOperator;
import org.openmbee.opensysml.proto.QueryResultElement;
import org.openmbee.opensysml.proto.RunAnalysisResponse;
import org.openmbee.opensysml.proto.UnitFactor;
import org.openmbee.opensysml.proto.UnitTerm;
import org.openmbee.opensysml.proto.ValidateInstanceResponse;
import org.openmbee.opensysml.proto.VerifyConstraintResponse;

/** Reading execution, verification, analysis and query answers off the wire, and writing requests. */
class ResultProtosTest {

  private static org.openmbee.opensysml.proto.Value integer(long value) {
    return org.openmbee.opensysml.proto.Value.newBuilder().setIntValue(value).build();
  }

  private static org.openmbee.opensysml.proto.Value real(double value) {
    return org.openmbee.opensysml.proto.Value.newBuilder().setRealValue(value).build();
  }

  @Test
  void aVerdictKeepsEveryFieldAndReadsAnAbsentObjectAsDeclaredValues() {
    var wire =
        org.openmbee.opensysml.proto.Verdict.newBuilder()
            .setKind("constraint")
            .setElementId("Demo::Vehicle::massLight")
            .setElement("massLight")
            .setHolds(false)
            .setCondition("mass < 100.0")
            .setRequirementId("Demo::Vehicle::lightEnough")
            .setInstancePath("sedan")
            .setEngine("interval")
            .setStrength("sound")
            .addBounds(Bound.newBuilder().setName("depth").setLimit(8).setReached(true))
            .build();
    Verdict verdict = Protos.verdict(wire);
    assertEquals(Verdict.KIND_CONSTRAINT, verdict.kind());
    assertEquals(Optional.of("Demo::Vehicle::massLight"), verdict.elementId());
    assertEquals("massLight", verdict.element());
    assertFalse(verdict.holds());
    assertTrue(verdict.decided());
    assertTrue(verdict.violated());
    assertEquals(Optional.of("mass < 100.0"), verdict.condition());
    assertEquals(Optional.empty(), verdict.instanceId());
    assertEquals(Optional.empty(), verdict.instanceTypeId());
    assertEquals(Optional.empty(), verdict.error());
    assertEquals(FailureReason.UNSPECIFIED, verdict.failureReason());
    assertEquals(Optional.of("Demo::Vehicle::lightEnough"), verdict.requirementId());
    assertEquals(Optional.of("sedan"), verdict.instancePath());
    assertEquals(
        new Standing("interval", "sound", List.of(new Standing.Bound("depth", 8, true))),
        verdict.standing());

    Verdict about =
        Protos.verdict(
            org.openmbee.opensysml.proto.Verdict.newBuilder()
                .setKind("object")
                .setInstanceId(3)
                .setInstanceTypeId("Demo::sedan")
                .setError("mass is unbound")
                .setFailureReason(
                    org.openmbee.opensysml.proto.FailureReason.FAILURE_REASON_EVALUATION)
                .build());
    assertEquals(Optional.of(3L), about.instanceId());
    assertEquals(Optional.of("Demo::sedan"), about.instanceTypeId());
    assertEquals(Optional.of("mass is unbound"), about.error());
    assertEquals(FailureReason.EVALUATION, about.failureReason());
    assertFalse(about.decided());
    assertFalse(about.standing().reported());
  }

  @Test
  void aFailureReasonThisReleaseDoesNotKnowIsUnknownRatherThanRefused() {
    assertEquals(
        FailureReason.UNKNOWN,
        Protos.failureReason(org.openmbee.opensysml.proto.FailureReason.UNRECOGNIZED));
    assertEquals(
        FailureReason.WRONG_KIND,
        Protos.failureReason(org.openmbee.opensysml.proto.FailureReason.FAILURE_REASON_WRONG_KIND));
    assertEquals(
        FailureReason.AMBIGUOUS_SUBJECT,
        Protos.failureReason(
            org.openmbee.opensysml.proto.FailureReason.FAILURE_REASON_AMBIGUOUS_SUBJECT));
  }

  @Test
  void aVerificationCarriesItsObjectsBodyVerdictsAndDiagnostics() {
    VerifyConstraintResponse response =
        VerifyConstraintResponse.newBuilder()
            .setVerdict(
                org.openmbee.opensysml.proto.Verdict.newBuilder()
                    .setKind("constraint")
                    .setHolds(true)
                    .setInstanceId(1))
            .addInstances(Instance.newBuilder().setId(1).setTypeSymbolId("Demo::sedan"))
            .addDiagnostics(Diagnostic.newBuilder().setMessage("note").setSeverity("info"))
            .build();
    Verification verification = Protos.verification(response);
    assertTrue(verification.holds());
    assertEquals("Demo::sedan", verification.subject().orElseThrow().typeSymbolId());
    assertEquals(1, verification.diagnostics().size());
    assertEquals("note", verification.diagnostics().get(0).message());
    assertEquals(List.of(), verification.verifications());
  }

  @Test
  void aValidationKeepsItsSummaryItsVerdictsAndWhetherItWasBounded() {
    ValidateInstanceResponse response =
        ValidateInstanceResponse.newBuilder()
            .setSummary(
                org.openmbee.opensysml.proto.Verdict.newBuilder()
                    .setKind("object")
                    .setHolds(false)
                    .setInstanceId(1))
            .addVerdicts(
                org.openmbee.opensysml.proto.Verdict.newBuilder()
                    .setKind("assertion")
                    .setHolds(false)
                    .setRequirementId("Demo::massTiny"))
            .addVerificationVerdicts(
                org.openmbee.opensysml.proto.VerificationVerdict.newBuilder()
                    .setCaseId("Demo::checkTiny")
                    .setKind("fail")
                    .setDetail("10 < 1200")
                    .setSubcase(true)
                    .setRequirementId("Demo::massTiny"))
            .addInstances(Instance.newBuilder().setId(1).setTypeSymbolId("Demo::sedan"))
            .setBounded(true)
            .build();
    Validation validation = Protos.validation(response);
    assertFalse(validation.holds());
    assertTrue(validation.bounded());
    assertEquals("Demo::sedan", validation.root().orElseThrow().typeSymbolId());
    assertEquals(1, validation.verdicts().size());
    VerificationVerdict body = validation.verifications().get(0);
    assertEquals(
        new VerificationVerdict(
            "Demo::checkTiny",
            VerificationVerdict.FAIL,
            Optional.of("10 < 1200"),
            true,
            Optional.of("Demo::massTiny")),
        body);
    assertFalse(body.passed());
  }

  @Test
  void aCalculationReadsADirectResultOrNamedOutputsInOrderWithItsStanding() {
    Calculation direct =
        Protos.calculation(EvaluateCalcResponse.newBuilder().setResult(integer(5)).build());
    assertEquals(Optional.of(new Value.IntegerValue(5)), direct.result());
    assertEquals(Optional.of(new Value.IntegerValue(5)), direct.value());
    assertFalse(direct.standing().reported());

    Calculation named =
        Protos.calculation(
            EvaluateCalcResponse.newBuilder()
                .addOutputs(CalcOutput.newBuilder().setName("z").setValue(real(1.5)))
                .addOutputs(CalcOutput.newBuilder().setName("a").setValue(real(2.5)))
                .setEngine("interval")
                .setStrength("sound")
                .addBounds(Bound.newBuilder().setName("runs").setLimit(100))
                .build());
    assertEquals(Optional.empty(), named.result());
    assertEquals(List.of("z", "a"), List.copyOf(named.outputs().keySet()));
    assertEquals(Optional.empty(), named.value());
    assertEquals("interval", named.standing().engine());
    assertEquals("sound", named.standing().strength());
    assertEquals(List.of(new Standing.Bound("runs", 100, false)), named.standing().bounds());
  }

  @Test
  void anAnalysisKeepsEachEvaluationItsErrorAndTheObjectsTheOutputsReferTo() {
    RunAnalysisResponse response =
        RunAnalysisResponse.newBuilder()
            .addOutputs(
                CalcOutput.newBuilder()
                    .setName("selectedAlternative")
                    .setValue(
                        org.openmbee.opensysml.proto.Value.newBuilder().setInstanceId(7)))
            .addVerdicts(
                org.openmbee.opensysml.proto.Verdict.newBuilder()
                    .setKind("objective")
                    .setElement("tradeStudyObjective")
                    .setHolds(true))
            .addEvaluations(
                CaseEvaluation.newBuilder()
                    .setFunctionId("Trade::lightest::evaluationFunction")
                    .addArguments(
                        org.openmbee.opensysml.proto.Value.newBuilder().setInstanceId(7))
                    .setResult(real(10.0))
                    .setSelected(true))
            .addEvaluations(
                CaseEvaluation.newBuilder()
                    .setFunctionId("Trade::lightest::evaluationFunction")
                    .addArguments(
                        org.openmbee.opensysml.proto.Value.newBuilder().setInstanceId(8))
                    .setError("division by zero")
                    .setTied(true))
            .addInstances(Instance.newBuilder().setId(7).setTypeSymbolId("Trade::b"))
            .addInstances(Instance.newBuilder().setId(8).setTypeSymbolId("Trade::c"))
            .setEngine("run")
            .setStrength("observed")
            .build();
    Analysis analysis = Protos.analysis(response);
    assertTrue(analysis.holds());
    assertEquals("tradeStudyObjective", analysis.objective().orElseThrow().element());
    assertEquals(2, analysis.evaluations().size());
    org.openmbee.opensysml.CaseEvaluation selected = analysis.selected().orElseThrow();
    assertEquals(List.of(new Value.InstanceReference(7)), selected.arguments());
    assertEquals(Optional.of(new Value.RealValue(10.0)), selected.result());
    assertEquals(Optional.empty(), selected.error());
    org.openmbee.opensysml.CaseEvaluation failed = analysis.evaluations().get(1);
    assertEquals(Optional.empty(), failed.result());
    assertEquals(Optional.of("division by zero"), failed.error());
    assertTrue(failed.tied());
    assertFalse(failed.selected());
    assertEquals(
        "Trade::b",
        analysis
            .resolve((Value.InstanceReference) analysis.outputs().get("selectedAlternative"))
            .orElseThrow()
            .typeSymbolId());
    assertEquals(new Standing("run", "observed", List.of()), analysis.standing());
  }

  @Test
  void aRunReportsItsFinalTimeOnlyWhenTheServiceDoes() {
    ExecuteActionResponse action =
        ExecuteActionResponse.newBuilder()
            .putOutputs("result", integer(5))
            .setFinalTime(2.5)
            .addDiagnostics(Diagnostic.newBuilder().setMessage("ran").setSeverity("info"))
            .build();
    ActionRun reported = Protos.actionRun(action, true);
    assertEquals(Map.of("result", new Value.IntegerValue(5)), reported.outputs());
    assertEquals(OptionalDouble.of(2.5), reported.finalTime());
    assertEquals(1, reported.diagnostics().size());
    assertEquals(OptionalDouble.empty(), Protos.actionRun(action, false).finalTime());

    ExecuteStateResponse state =
        ExecuteStateResponse.newBuilder()
            .addStatesVisited("init")
            .addStatesVisited("Running")
            .putFinalContext("n", integer(1))
            .setFinalTime(0.0)
            .build();
    StateRun run = Protos.stateRun(state, true);
    assertEquals(List.of("init", "Running"), run.statesVisited());
    assertEquals(Optional.of("Running"), run.finalState());
    assertEquals(Map.of("n", new Value.IntegerValue(1)), run.finalContext());
    assertEquals(OptionalDouble.of(0.0), run.finalTime());
  }

  @Test
  void anExplorationKeepsEveryOutcomeItsWitnessAndHowTheSearchEnded() {
    List<Outcome> outcomes =
        List.of(
            Outcome.newBuilder()
                .putOutputs("x", integer(1))
                .setLinearizations(2)
                .addWitness("first of a, b, c: a")
                .setProbability(0.5)
                .addDiagnostics(Diagnostic.newBuilder().setMessage("choice").setSeverity("info"))
                .build(),
            Outcome.newBuilder()
                .setFinalState("done")
                .addStatesVisited("init")
                .addStatesVisited("done")
                .setLinearizations(1)
                .build(),
            Outcome.newBuilder().setError("deadlock").setLinearizations(1).build());
    ExplorationStatus status =
        ExplorationStatus.newBuilder()
            .setComplete(false)
            .setRuns(100)
            .addBudgetsHit("runs")
            .setRunsBudget(100)
            .setDepthBudget(64)
            .setProbabilitiesLowerBound(true)
            .build();
    Exploration exploration = Protos.exploration(outcomes, status);
    assertEquals(3, exploration.outcomes().size());
    org.openmbee.opensysml.Outcome first = exploration.outcomes().get(0);
    assertEquals(Map.of("x", new Value.IntegerValue(1)), first.outputs());
    assertEquals(2, first.linearizations());
    assertEquals(0.5, first.probability());
    assertEquals(List.of("first of a, b, c: a"), first.witness());
    assertEquals(1, first.diagnostics().size());
    assertTrue(first.completed());
    org.openmbee.opensysml.Outcome machine = exploration.outcomes().get(1);
    assertEquals(Optional.of("done"), machine.finalState());
    assertEquals(List.of("init", "done"), machine.statesVisited());
    org.openmbee.opensysml.Outcome failed = exploration.outcomes().get(2);
    assertEquals(Optional.of("deadlock"), failed.error());
    assertFalse(failed.completed());
    assertFalse(exploration.complete());
    assertEquals(100, exploration.runs());
    assertEquals(List.of("runs"), exploration.budgetsHit());
    assertTrue(exploration.probabilitiesLowerBound());
    assertEquals(
        "incomplete: runs budget 100 hit after 100 runs; probabilities are lower bounds",
        exploration.status());
  }

  @Test
  void aQueryWritesItsScopeSelectionAndNestedConditions() {
    Query query =
        Query.all()
            .withScope(List.of("Demo"))
            .withSelect(List.of("name"))
            .where(
                Condition.all(
                    List.of(
                        Condition.equalTo("@type", List.of("PartUsage", "PartDefinition")).negated(),
                        Condition.any(
                            List.of(
                                Condition.greater("mass", "1000"),
                                Condition.less("mass", "10"))))));
    org.openmbee.opensysml.proto.Query wire = Protos.proto(query);
    assertEquals(List.of("Demo"), wire.getScopeList());
    assertEquals(List.of("name"), wire.getSelectList());
    Constraint where = wire.getWhere();
    assertEquals(Constraint.ConstraintCase.COMPOSITE, where.getConstraintCase());
    assertEquals(CompositeOperator.COMPOSITE_OPERATOR_AND, where.getComposite().getOperator());
    var type = where.getComposite().getConstraint(0).getPrimitive();
    assertTrue(type.getInverse());
    assertEquals("@type", type.getProperty());
    assertEquals(PrimitiveOperator.PRIMITIVE_OPERATOR_EQUAL, type.getOperator());
    assertEquals(List.of("PartUsage", "PartDefinition"), type.getValueList());
    var mass = where.getComposite().getConstraint(1).getComposite();
    assertEquals(CompositeOperator.COMPOSITE_OPERATOR_OR, mass.getOperator());
    assertEquals(
        PrimitiveOperator.PRIMITIVE_OPERATOR_GREATER, mass.getConstraint(0).getPrimitive().getOperator());
    assertEquals(List.of("1000"), mass.getConstraint(0).getPrimitive().getValueList());
    assertEquals(
        PrimitiveOperator.PRIMITIVE_OPERATOR_LESS, mass.getConstraint(1).getPrimitive().getOperator());
    assertFalse(Protos.proto(Query.all()).hasWhere());
  }

  @Test
  void queryElementsKeepTheirIdsTypesAndSelectedProperties() {
    List<QueryElement> elements =
        Protos.queryElements(
            List.of(
                QueryResultElement.newBuilder()
                    .setId("Demo::sedan")
                    .setType("PartUsage")
                    .putProperties("name", "sedan")
                    .build(),
                QueryResultElement.newBuilder().setId("Demo::Wheel").setType("PartDefinition").build()));
    assertEquals(
        List.of(
            new QueryElement("Demo::sedan", "PartUsage", Map.of("name", "sedan")),
            new QueryElement("Demo::Wheel", "PartDefinition", Map.of())),
        elements);
  }

  @Test
  void engineDescriptionsKeepEveryField() {
    List<EngineInfo> engines =
        Protos.engines(
            List.of(
                org.openmbee.opensysml.proto.EngineInfo.newBuilder()
                    .setName("interval")
                    .setAuthority("sound")
                    .addAnswers("constraint")
                    .addBounds("depth")
                    .setProcess("sysml-interval")
                    .setProcessFound("/usr/bin/sysml-interval")
                    .setReady(true)
                    .setKind("external")
                    .setProtocol("stdio")
                    .setSource("Engines::interval")
                    .setCommand("sysml-interval --serve")
                    .setVersion("1.2.0")
                    .setServed(true)
                    .build()));
    EngineInfo engine = engines.get(0);
    assertEquals("interval", engine.name());
    assertEquals("sound", engine.authority());
    assertEquals(List.of("constraint"), engine.answers());
    assertEquals(List.of("depth"), engine.bounds());
    assertEquals("sysml-interval", engine.process());
    assertEquals("/usr/bin/sysml-interval", engine.processFound());
    assertTrue(engine.ready());
    assertEquals("", engine.unavailable());
    assertEquals("external", engine.kind());
    assertEquals("stdio", engine.protocol());
    assertEquals("Engines::interval", engine.source());
    assertEquals("sysml-interval --serve", engine.command());
    assertEquals("1.2.0", engine.version());
    assertTrue(engine.served());
  }

  @Test
  void aRequestValueSurvivesTheRoundTripThroughTheWire() {
    UnitTerm metre =
        UnitTerm.newBuilder()
            .addFactors(UnitFactor.newBuilder().setUnitId("SI::metre").setExponent(1))
            .setScaleNum(1000)
            .setScaleDen(1)
            .build();
    List<Value> values =
        List.of(
            new Value.IntegerValue(3),
            new Value.RealValue(2.5),
            new Value.BooleanValue(true),
            new Value.StringValue("four"),
            new Value.ComplexValue(1.0, -2.0),
            new Value.InstanceReference(7),
            new Value.NullValue(),
            new Value.Sequence(List.of(new Value.IntegerValue(1), new Value.StringValue("a"))),
            new Value.VectorValue(List.of(new Value.RealValue(3.0), new Value.RealValue(4.0))),
            new Value.QuantityValue(
                Protos.quantity(
                    org.openmbee.opensysml.proto.Quantity.newBuilder()
                        .setIntMagnitude(3)
                        .setUnit("km")
                        .setUnitTerm(metre)
                        .build())));
    for (Value value : values) {
      org.openmbee.opensysml.proto.Value wire = Protos.proto(value);
      assertEquals(Optional.of(value), Protos.value(wire), value.toString());
    }
    List<org.openmbee.opensysml.proto.Value> wires = Protos.protos(values);
    assertEquals(values.size(), wires.size());
    Map<String, org.openmbee.opensysml.proto.Value> named =
        Protos.protos(Map.of("limit", new Value.RealValue(50.0)));
    assertEquals(50.0, named.get("limit").getRealValue());
    Quantity quantity = ((Value.QuantityValue) values.get(9)).quantity();
    assertEquals(Optional.of("km"), quantity.unit());
    assertThrows(NullPointerException.class, () -> Protos.proto((Value) null));
  }
}
