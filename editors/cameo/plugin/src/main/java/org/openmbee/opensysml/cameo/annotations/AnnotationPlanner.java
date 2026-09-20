package org.openmbee.opensysml.cameo.annotations;

import java.util.List;
import org.openmbee.opensysml.cameo.identity.IdentityResolver;
import org.openmbee.opensysml.cameo.results.RunResult;

/** Turns outcome rows into annotation plans for every Cameo element the outcome resolves to. */
public final class AnnotationPlanner {
  private AnnotationPlanner() {}

  public static List<AnnotationPlan> plan(RunResult result, IdentityResolver resolver) {
    return result.outcomes().stream()
        .filter(outcome -> outcome.elementId() != null)
        .flatMap(outcome -> resolver.resolve(outcome.elementId()).stream()
            .map(target -> new AnnotationPlan(target, severity(outcome.status()), result.operation().label(), outcome.detail())))
        .toList();
  }

  static AnnotationPlan.Severity severity(RunResult.Status status) {
    return switch (status) {
      case PASSED, INFO -> AnnotationPlan.Severity.INFO;
      case FAILED -> AnnotationPlan.Severity.ERROR;
      case INCONCLUSIVE, ERROR, CANCELLED -> AnnotationPlan.Severity.WARNING;
    };
  }
}
