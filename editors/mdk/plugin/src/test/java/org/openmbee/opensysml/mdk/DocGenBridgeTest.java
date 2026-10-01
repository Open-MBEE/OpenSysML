package org.openmbee.opensysml.mdk;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertThrows;

import java.time.Duration;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import org.junit.jupiter.api.Test;
import org.openmbee.opensysml.Diagnostic;
import org.openmbee.opensysml.mdk.bridge.DocGenBridge;
import org.openmbee.opensysml.mdk.engine.Operation;
import org.openmbee.opensysml.mdk.model.ModelPath;
import org.openmbee.opensysml.mdk.results.RunResult;

class DocGenBridgeTest {
  @Test
  void flattensARunResultIntoJdkValuesOnly() {
    RunResult result = new RunResult(
        Operation.VERIFY, ModelPath.V1_MDZIP, "Model::Req", RunResult.Status.FAILED,
        List.of(new RunResult.Outcome("req1", "mass <= 10", RunResult.Status.FAILED, "e1"),
            new RunResult.Outcome("req2", "ok", RunResult.Status.PASSED, null)),
        List.of(new Diagnostic(Diagnostic.Severity.WARNING, "two guards hold", "choice-point",
            Optional.of(new Diagnostic.Span("a.sysml", 3, 1, 3, 9)))),
        Optional.of("12 s"), List.of("start", "done"), Duration.ofMillis(42));
    Map<String, Object> flat = DocGenBridge.flatten(result);
    assertEquals("VERIFY", flat.get("operation"));
    assertEquals("Verify", flat.get("operationLabel"));
    assertEquals("V1_MDZIP", flat.get("path"));
    assertEquals("Model::Req", flat.get("subject"));
    assertEquals("FAILED", flat.get("status"));
    assertEquals(42L, flat.get("elapsedMillis"));
    assertEquals("12 s", flat.get("finalTime"));
    assertEquals(List.of(
        Map.of("label", "req1", "detail", "mass <= 10", "status", "FAILED", "elementId", "e1"),
        Map.of("label", "req2", "detail", "ok", "status", "PASSED")), flat.get("outcomes"));
    assertEquals(List.of(Map.of(
        "severity", "WARNING", "code", "choice-point", "message", "two guards hold", "location", "a.sysml:3:1")),
        flat.get("diagnostics"));
    assertEquals(List.of("start", "done"), flat.get("schedule"));
    flat.values().forEach(value -> assertFalse(value.getClass().getName().startsWith("org.openmbee"), value.toString()));
  }

  @Test
  void omitsFinalTimeWhenAbsent() {
    RunResult result = new RunResult(
        Operation.INSTANTIATE, ModelPath.V2_TEXTUAL, "P", RunResult.Status.PASSED,
        List.of(), List.of(), Optional.empty(), List.of(), Duration.ZERO);
    assertFalse(DocGenBridge.flatten(result).containsKey("finalTime"));
  }

  @Test
  void resolvesOperationsByEnumName() {
    assertEquals(Operation.EVALUATE_CALC, DocGenBridge.operation("EVALUATE_CALC"));
    IllegalArgumentException failure =
        assertThrows(IllegalArgumentException.class, () -> DocGenBridge.operation("Evaluate calc"));
    assertEquals("unknown OpenSysML operation: Evaluate calc", failure.getMessage());
  }
}
