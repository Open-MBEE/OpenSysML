package org.openmbee.opensysml;

import java.util.Objects;
import java.util.Optional;

/**
 * What the body of a verification case answered when it ran, which is a separate answer from
 * whether the requirement it verifies is satisfied. Reported by a service advertising the {@code
 * verification_verdicts} capability.
 *
 * @param caseId FQN of the verification case that ran
 * @param kind the verdict its body produced: {@link #PASS}, {@link #FAIL}, {@link #INCONCLUSIVE} or
 *     {@link #ERROR}
 * @param detail the text of an error verdict, or why an inconclusive one decided nothing; absent
 *     for a pass and a fail
 * @param subcase whether this is the verdict of a case another performed, reported on its own
 *     because the library states no roll-up for it
 * @param requirementId FQN of the requirement the verdict was reported for, absent when the case
 *     ran for itself
 */
public record VerificationVerdict(
    String caseId, String kind, Optional<String> detail, boolean subcase, Optional<String> requirementId) {

  /** A body whose verdict value is {@code VerdictKind::pass}. */
  public static final String PASS = "pass";

  /** A body whose verdict value is {@code VerdictKind::fail}. */
  public static final String FAIL = "fail";

  /** A body that ran and produced no verdict value. */
  public static final String INCONCLUSIVE = "inconclusive";

  /** A body whose run could not be carried out. */
  public static final String ERROR = "error";

  /**
   * Creates a verification verdict.
   *
   * @param caseId the case's FQN, never {@code null}
   * @param kind the verdict kind, never {@code null}
   * @param detail the detail, when there is one
   * @param subcase whether it is a subcase's
   * @param requirementId the requirement, when reported for one
   */
  public VerificationVerdict {
    Objects.requireNonNull(caseId, "caseId");
    Objects.requireNonNull(kind, "kind");
    Objects.requireNonNull(detail, "detail");
    Objects.requireNonNull(requirementId, "requirementId");
  }

  /**
   * Whether the body passed.
   *
   * @return {@code true} for a {@link #PASS} verdict
   */
  public boolean passed() {
    return PASS.equals(kind);
  }
}
