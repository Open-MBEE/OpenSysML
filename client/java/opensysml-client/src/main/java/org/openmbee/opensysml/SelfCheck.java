package org.openmbee.opensysml;

import java.util.Collections;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Objects;

/**
 * What {@link Model#selfCheck} found: every rule constraint of the {@code SysMLValidation}
 * library applied to every element it covers, as {@code sysml -self-check} reports.
 *
 * @param status {@code holds} when nothing was violated and nothing errored, else {@code fails}
 * @param findings each violation, unevaluated check, error and skipped rule constraint
 * @param elements how many elements were checked
 * @param checks how many checks were made
 * @param violations how many checks found a violation
 * @param unevaluated how many checks the runtime could not evaluate
 * @param evaluationErrors how many checks errored
 * @param applications how many times each constraint was applied, in library order
 * @param summary the closing line {@code sysml -self-check} prints
 * @param diagnostics notes raised
 */
public record SelfCheck(
    String status,
    List<Finding> findings,
    long elements,
    long checks,
    long violations,
    long unevaluated,
    long evaluationErrors,
    Map<String, Long> applications,
    String summary,
    List<Diagnostic> diagnostics) {
  /** Validates and defensively copies the collections. */
  public SelfCheck {
    Objects.requireNonNull(status, "status");
    findings = List.copyOf(findings);
    applications = Collections.unmodifiableMap(new LinkedHashMap<>(applications));
    Objects.requireNonNull(summary, "summary");
    diagnostics = List.copyOf(diagnostics);
  }

  /**
   * Whether the check found no violation and no evaluation error.
   *
   * @return true if the status is {@code holds}
   */
  public boolean holds() {
    return "holds".equals(status);
  }

  /**
   * One thing the check reports.
   *
   * @param status {@code fails}, {@code unevaluated}, {@code error} or {@code warning}
   * @param subject {@code constraint for element}, or the constraint or package alone
   * @param location {@code document:line:col} of the element; empty for none
   * @param constraint the constraint applied
   * @param element the element it was applied to
   * @param message the reason, for a warning or an error
   * @param line the finding as {@code sysml -self-check} prints it
   */
  public record Finding(
      String status,
      String subject,
      String location,
      String constraint,
      String element,
      String message,
      String line) {
    /** Validates the fields. */
    public Finding {
      Objects.requireNonNull(status, "status");
      Objects.requireNonNull(subject, "subject");
      Objects.requireNonNull(location, "location");
      Objects.requireNonNull(constraint, "constraint");
      Objects.requireNonNull(element, "element");
      Objects.requireNonNull(message, "message");
      Objects.requireNonNull(line, "line");
    }
  }
}
