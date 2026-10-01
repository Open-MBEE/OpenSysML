package org.openmbee.opensysml.mdk.docgen;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import org.junit.jupiter.api.Test;

class BridgeResultTest {
  @Test
  void readsEveryFieldOfTheFlatMap() {
    BridgeResult result = BridgeResult.from(Flat.verify("Model::Req", "FAILED"));
    assertEquals("VERIFY", result.operation());
    assertEquals("Verify", result.operationLabel());
    assertEquals("Model::Req", result.subject());
    assertEquals("FAILED", result.status());
    assertEquals(42L, result.elapsedMillis());
    assertEquals(Optional.empty(), result.finalTime());
    assertEquals(List.of(
        new BridgeResult.Outcome("req1", "mass <= 10", "FAILED", Optional.of("e1")),
        new BridgeResult.Outcome("req2", "power <= 5", "PASSED", Optional.empty())), result.outcomes());
    assertEquals(List.of(new BridgeResult.Diagnostic(
        "WARNING", "choice-point", "two guards hold", Optional.of("a.sysml:3:1"))), result.diagnostics());
    assertEquals(List.of("start", "check", "done"), result.schedule());
  }

  @Test
  void toleratesAbsentOptionalSectionsAndKeepsFinalTime() {
    Map<String, Object> flat = new LinkedHashMap<>();
    flat.put("operation", "EXECUTE_STATE");
    flat.put("operationLabel", "Execute state");
    flat.put("subject", "S");
    flat.put("status", "INFO");
    flat.put("finalTime", "12.5 s");
    BridgeResult result = BridgeResult.from(flat);
    assertEquals(0L, result.elapsedMillis());
    assertEquals(Optional.of("12.5 s"), result.finalTime());
    assertTrue(result.outcomes().isEmpty());
    assertTrue(result.diagnostics().isEmpty());
    assertTrue(result.schedule().isEmpty());
  }

  @Test
  void rejectsMapsMissingTheContractKeys() {
    Map<String, Object> flat = Flat.verify("S", "PASSED");
    flat.remove("status");
    IllegalArgumentException failure =
        assertThrows(IllegalArgumentException.class, () -> BridgeResult.from(flat));
    assertEquals("bridge result lacks status", failure.getMessage());
  }
}
