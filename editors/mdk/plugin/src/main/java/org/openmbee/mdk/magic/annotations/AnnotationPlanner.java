package org.openmbee.mdk.magic.annotations;

import java.util.ArrayList;
import java.util.HashSet;
import java.util.List;
import java.util.Set;
import org.openmbee.mdk.magic.identity.IdentityResolver;
import org.openmbee.mdk.magic.model.ModelElement;
import org.openmbee.mdk.magic.results.RunResult;

/** Turns outcome rows into annotation plans for every Cameo element the outcome resolves to. */
public final class AnnotationPlanner {
  private AnnotationPlanner() {}

  public static List<AnnotationPlan> plan(RunResult result, IdentityResolver resolver) {
    List<AnnotationPlan> plans = new ArrayList<>(result.outcomes().stream()
        .filter(outcome -> outcome.elementId() != null)
        .flatMap(outcome -> resolver.resolve(outcome.elementId()).stream()
            .map(target -> new AnnotationPlan(target, severity(outcome.status()), result.operation().label(), outcome.detail())))
        .toList());
    plans.addAll(rows(result, resolver));
    return List.copyOf(plans);
  }

  /** One informational annotation per element a query row was computed for, carrying the row's cells. */
  static List<AnnotationPlan> rows(RunResult result, IdentityResolver resolver) {
    List<AnnotationPlan> plans = new ArrayList<>();
    Set<String> seen = new HashSet<>();
    for (RunResult.Table table : result.tables()) {
      for (RunResult.Row row : table.rows()) {
        if (row.elementId() == null || !seen.add(table.query() + "\u0000" + row.elementId())) continue;
        String detail = table.query() + ": " + cells(table, row);
        for (ModelElement target : resolver.resolve(row.elementId())) {
          plans.add(new AnnotationPlan(target, AnnotationPlan.Severity.INFO, result.operation().label(), detail));
        }
      }
    }
    return plans;
  }

  private static String cells(RunResult.Table table, RunResult.Row row) {
    StringBuilder text = new StringBuilder();
    for (int i = 0; i < table.columns().size() && i < row.cells().size(); i++) {
      if (i > 0) text.append(", ");
      text.append(table.columns().get(i)).append(" = ").append(row.cells().get(i).text());
    }
    return text.toString();
  }

  static AnnotationPlan.Severity severity(RunResult.Status status) {
    return switch (status) {
      case PASSED, INFO -> AnnotationPlan.Severity.INFO;
      case FAILED -> AnnotationPlan.Severity.ERROR;
      case INCONCLUSIVE, ERROR, CANCELLED -> AnnotationPlan.Severity.WARNING;
    };
  }
}
