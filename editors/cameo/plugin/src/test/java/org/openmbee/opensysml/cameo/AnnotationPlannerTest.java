package org.openmbee.opensysml.cameo;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;

import java.time.Duration;
import java.util.List;
import java.util.Optional;
import org.junit.jupiter.api.Test;
import org.openmbee.opensysml.cameo.annotations.AnnotationPlan;
import org.openmbee.opensysml.cameo.annotations.AnnotationPlanner;
import org.openmbee.opensysml.cameo.engine.Operation;
import org.openmbee.opensysml.cameo.identity.IdentityIndex;
import org.openmbee.opensysml.cameo.model.ModelPath;
import org.openmbee.opensysml.cameo.model.SimpleElement;
import org.openmbee.opensysml.cameo.results.RunResult;
import org.openmbee.opensysml.cameo.results.RunResult.Outcome;
import org.openmbee.opensysml.cameo.results.RunResult.Status;

class AnnotationPlannerTest {
  private static RunResult result(Outcome... outcomes) {
    return new RunResult(Operation.VERIFY, ModelPath.V2_TEXTUAL, "Demo::c", Status.FAILED,
        List.of(outcomes), List.of(), Optional.empty(), List.of(), Duration.ZERO);
  }

  @Test
  void mapsStatusesToSeverities() {
    var index = IdentityIndex.of(List.of(
        new SimpleElement("1", "Demo::c", "c", "c"),
        new SimpleElement("2", "Demo::d", "d", "d"),
        new SimpleElement("3", "Demo::e", "e", "e")));
    var plans = AnnotationPlanner.plan(result(
        new Outcome("Demo::c", "failed", Status.FAILED, "Demo::c"),
        new Outcome("Demo::d", "ok", Status.PASSED, "Demo::d"),
        new Outcome("Demo::e", "boom", Status.ERROR, "Demo::e"),
        new Outcome("out", "1", Status.INFO, null),
        new Outcome("Demo::missing", "?", Status.FAILED, "Demo::missing")), index);
    assertEquals(3, plans.size());
    assertEquals(AnnotationPlan.Severity.ERROR, plans.get(0).severity());
    assertEquals(AnnotationPlan.Severity.INFO, plans.get(1).severity());
    assertEquals(AnnotationPlan.Severity.WARNING, plans.get(2).severity());
    assertEquals("Verify", plans.get(0).kind());
    assertEquals("c", plans.get(0).target().handle());
  }

  @Test
  void ambiguousNamesAnnotateEveryCandidate() {
    var index = IdentityIndex.of(List.of(
        new SimpleElement("1", "Demo::c", "c", "a"), new SimpleElement("2", "Demo::c", "c", "b")));
    var plans = AnnotationPlanner.plan(result(new Outcome("Demo::c", "failed", Status.FAILED, "Demo::c")), index);
    assertEquals(2, plans.size());
    assertTrue(plans.stream().allMatch(plan -> plan.severity() == AnnotationPlan.Severity.ERROR));
  }
}
