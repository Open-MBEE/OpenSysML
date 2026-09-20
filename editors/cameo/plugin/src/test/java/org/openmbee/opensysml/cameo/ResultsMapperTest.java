package org.openmbee.opensysml.cameo;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.openmbee.opensysml.cameo.TestRequests.request;

import java.time.Duration;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.OptionalDouble;
import org.junit.jupiter.api.Test;
import org.openmbee.opensysml.ActionRun;
import org.openmbee.opensysml.Analysis;
import org.openmbee.opensysml.Calculation;
import org.openmbee.opensysml.Diagnostic;
import org.openmbee.opensysml.FailureReason;
import org.openmbee.opensysml.Instance;
import org.openmbee.opensysml.Instantiation;
import org.openmbee.opensysml.Satisfaction;
import org.openmbee.opensysml.Standing;
import org.openmbee.opensysml.StateRun;
import org.openmbee.opensysml.Validation;
import org.openmbee.opensysml.Value;
import org.openmbee.opensysml.Verdict;
import org.openmbee.opensysml.Verification;
import org.openmbee.opensysml.VerificationVerdict;
import org.openmbee.opensysml.cameo.engine.Operation;
import org.openmbee.opensysml.cameo.results.ResultsMapper;
import org.openmbee.opensysml.cameo.results.RunResult;
import org.openmbee.opensysml.cameo.results.RunResult.Status;

class ResultsMapperTest {
  private static final Duration ELAPSED = Duration.ofMillis(5);

  private static Verdict verdict(String element, boolean holds, Optional<String> error) {
    return new Verdict(
        Verdict.KIND_CONSTRAINT, Optional.of(element), element, holds, Optional.of("x > 0"),
        Optional.empty(), Optional.empty(), error, FailureReason.UNSPECIFIED, Optional.empty(),
        Optional.empty(), Standing.none());
  }

  @Test
  void verificationVerdictDrivesStatus() {
    var passed = ResultsMapper.map(request(Operation.VERIFY, "A::c"),
        new Verification(verdict("A::c", true, Optional.empty()), List.of(), List.of(), List.of()), ELAPSED);
    assertEquals(Status.PASSED, passed.status());
    assertEquals("A::c", passed.outcomes().get(0).elementId());
    assertEquals(ELAPSED, passed.elapsed());
    var failed = ResultsMapper.map(request(Operation.VERIFY, "A::c"),
        new Verification(verdict("A::c", false, Optional.empty()), List.of(), List.of(), List.of()), ELAPSED);
    assertEquals(Status.FAILED, failed.status());
    var errored = ResultsMapper.map(request(Operation.VERIFY, "A::c"),
        new Verification(verdict("A::c", false, Optional.of("boom")), List.of(), List.of(), List.of()), ELAPSED);
    assertEquals(Status.ERROR, errored.status());
    assertEquals("boom", errored.outcomes().get(0).detail());
  }

  @Test
  void verificationCaseRowsBecomeOutcomes() {
    var row = new VerificationVerdict("A::case", VerificationVerdict.FAIL, Optional.of("too heavy"), false, Optional.of("A::req"));
    var result = ResultsMapper.map(request(Operation.VERIFY, "A::req"),
        new Verification(verdict("A::req", false, Optional.empty()), List.of(row), List.of(), List.of()), ELAPSED);
    assertEquals(2, result.outcomes().size());
    assertEquals("A::req", result.outcomes().get(1).elementId());
    assertEquals(Status.FAILED, result.outcomes().get(1).status());
    var undecided = new VerificationVerdict("A::case", VerificationVerdict.INCONCLUSIVE, Optional.empty(), false, Optional.empty());
    var open = ResultsMapper.map(request(Operation.VERIFY, "A::req"),
        new Verification(verdict("A::req", true, Optional.empty()), List.of(undecided), List.of(), List.of()), ELAPSED);
    assertEquals(Status.INCONCLUSIVE, open.outcomes().get(1).status());
  }

  @Test
  void stateRunExposesScheduleAndFinalTime() {
    var run = new StateRun(List.of("Off", "On"), Map.of("count", new Value.IntegerValue(2)), OptionalDouble.of(3.5), List.of());
    var result = ResultsMapper.map(request(Operation.EXECUTE_STATE, "A::sm"), run, ELAPSED);
    assertEquals(List.of("Off", "On"), result.schedule());
    assertEquals(Optional.of("3.5"), result.finalTime());
    assertEquals("in On", result.outcomes().get(0).detail());
    assertEquals("2", result.outcomes().get(1).detail());
  }

  @Test
  void actionRunKeepsOutputsAndDiagnostics() {
    var diagnostic = new Diagnostic(Diagnostic.Severity.WARNING, "slow", "w1", Optional.empty());
    var run = new ActionRun(Map.of("out", new Value.RealValue(1.5)), OptionalDouble.empty(), List.of(diagnostic));
    var result = ResultsMapper.map(request(Operation.EXECUTE_ACTION, "A::act"), run, ELAPSED);
    assertEquals(Status.PASSED, result.status());
    assertEquals(Optional.empty(), result.finalTime());
    assertEquals(List.of(diagnostic), result.diagnostics());
    assertEquals("1.5", result.outcomes().get(1).detail());
  }

  @Test
  void instantiationListsInstances() {
    var root = new Instance(1, "A::Vehicle", Map.of());
    var wheel = new Instance(2, "A::Wheel", Map.of());
    var result = ResultsMapper.map(request(Operation.INSTANTIATE, "A::Vehicle"),
        new Instantiation(root, List.of(root, wheel), List.of()), ELAPSED);
    assertEquals(3, result.outcomes().size());
    assertEquals("A::Wheel", result.outcomes().get(2).elementId());
  }

  @Test
  void calculationRendersResult() {
    var result = ResultsMapper.map(request(Operation.EVALUATE_CALC, "A::f"),
        new Calculation(Optional.of(new Value.IntegerValue(42)), Map.of(), List.of(), Standing.none()), ELAPSED);
    assertEquals("42", result.outcomes().get(0).detail());
    var empty = ResultsMapper.map(request(Operation.EVALUATE_CALC, "A::f"),
        new Calculation(Optional.empty(), Map.of(), List.of(), Standing.none()), ELAPSED);
    assertEquals("no result value", empty.outcomes().get(0).detail());
    var single = ResultsMapper.map(request(Operation.EVALUATE_CALC, "A::f"),
        new Calculation(Optional.empty(), Map.of("out", new Value.IntegerValue(7)), List.of(), Standing.none()), ELAPSED);
    assertEquals("7", single.outcomes().get(0).detail());
  }

  @Test
  void analysisAndSatisfactionAggregateVerdicts() {
    var analysis = new Analysis(Map.of(), List.of(verdict("A::obj", false, Optional.empty())), List.of(), List.of(), List.of(), List.of(), Standing.none());
    var analysed = ResultsMapper.map(request(Operation.RUN_ANALYSIS, "A::case"), analysis, ELAPSED);
    assertEquals(Status.FAILED, analysed.status());
    assertEquals("A::case", analysed.outcomes().get(0).elementId());
    var satisfaction = new Satisfaction(List.of(verdict("A::req", true, Optional.empty())), List.of(), List.of(), List.of());
    assertEquals(Status.PASSED, ResultsMapper.map(request(Operation.VERIFY, "A::sat"), satisfaction, ELAPSED).status());
    var broken = new Satisfaction(List.of(verdict("A::req", true, Optional.of("bad"))), List.of(), List.of(), List.of());
    assertEquals(Status.ERROR, ResultsMapper.map(request(Operation.VERIFY, "A::sat"), broken, ELAPSED).status());
  }

  @Test
  void validationFollowsSummary() {
    var validation = new Validation(verdict("A::v", true, Optional.empty()), List.of(), List.of(), List.of(), List.of(), true);
    assertEquals(Status.PASSED, ResultsMapper.map(request(Operation.VERIFY, "A::v"), validation, ELAPSED).status());
  }

  @Test
  void leadingDiagnosticsAndErrorsAreRepresented() {
    var request = request(Operation.VERIFY, "A::c");
    var error = RunResult.error(request, new IllegalStateException("no such symbol"), ELAPSED);
    assertEquals(Status.ERROR, error.status());
    assertEquals("no such symbol", error.outcomes().get(0).detail());
    var parse = new Diagnostic(Diagnostic.Severity.ERROR, "unexpected token", "p1", Optional.empty());
    var merged = error.withLeadingDiagnostics(List.of(parse));
    assertEquals(parse, merged.diagnostics().get(0));
    assertTrue(RunResult.cancelled(request).outcomes().isEmpty());
    assertEquals(Status.CANCELLED, RunResult.cancelled(request).status());
  }
}
