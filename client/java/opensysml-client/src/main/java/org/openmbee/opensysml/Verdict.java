package org.openmbee.opensysml;

import java.util.List;
import java.util.Objects;
import java.util.Optional;

/**
 * One verification's answer: whether a condition held and, when it did not, what the model answered
 * false about.
 *
 * <p>{@code holds} false with {@code error} absent is the model's answer of false; false with an
 * error present is no answer at all, see {@link #decided()}. Neither is an exception: a verdict is
 * a result to read, and only a call the service could not answer throws {@link ModelException}.
 *
 * @param kind what was verified: {@link #KIND_CONSTRAINT}, {@link #KIND_REQUIREMENT}, {@link
 *     #KIND_SATISFY}, {@link #KIND_OBJECTIVE}, {@link #KIND_ASSERTION} or {@link #KIND_OBJECT}
 * @param elementId FQN of the element verified, absent for an anonymous satisfy assertion
 * @param element the element as a reader names it, or the assertion as written
 * @param holds the model's answer, meaningful only when {@link #decided()}
 * @param condition the condition that evaluated to false, as written, when the runtime names one
 * @param instanceId id of the object the verdict is about, absent when it is about declared values
 *     alone; the result carrying the verdict resolves it to an {@link Instance}
 * @param instanceTypeId FQN of that object's type, absent with {@code instanceId}
 * @param error why evaluation failed, present only when nothing was decided
 * @param failureReason what kind of failure {@code error} reports
 * @param requirementId FQN of the requirement a satisfaction verdict asserts satisfied, absent when
 *     the assertion names none
 * @param instancePath the features held from a validated object to the one this verdict is about
 *     ({@code "engine.pump"}, {@code "wheels[2]"}), absent for the validated object itself and
 *     outside a {@link Validation}
 * @param question the question the verdict answers: {@code "evaluate"} for an evaluation, absent
 *     from a service predating {@code verification_questions}
 * @param status the answer's status: {@code holds} | {@code violated} | {@code undecided} |
 *     {@code satisfiable} | {@code unsatisfiable}, as the service spells it
 * @param witness the assignment witnessing the answer: the free features' values for a violated
 *     {@code holds} question or a satisfiable {@code satisfiable} question
 * @param standing how strongly the verdict stands
 */
public record Verdict(
    String kind,
    Optional<String> elementId,
    String element,
    boolean holds,
    Optional<String> condition,
    Optional<Long> instanceId,
    Optional<String> instanceTypeId,
    Optional<String> error,
    FailureReason failureReason,
    Optional<String> requirementId,
    Optional<String> instancePath,
    Optional<String> question,
    Optional<String> status,
    List<WitnessAssignment> witness,
    Standing standing) {

  /** A constraint verified against declared or an object's values. */
  public static final String KIND_CONSTRAINT = "constraint";

  /** A requirement's require constraints verified. */
  public static final String KIND_REQUIREMENT = "requirement";

  /** A {@code satisfy} assertion. */
  public static final String KIND_SATISFY = "satisfy";

  /** An analysis case's objective. */
  public static final String KIND_OBJECTIVE = "objective";

  /** An {@code assert constraint} in a case's body. */
  public static final String KIND_ASSERTION = "assertion";

  /** A validated object as a whole. */
  public static final String KIND_OBJECT = "object";

  /** The evaluation a verification asks unless a question names another. */
  public static final String QUESTION_EVALUATE = "evaluate";

  /** The question whether the claim holds for every free assignment. */
  public static final String QUESTION_HOLDS = "holds";

  /** The question whether any free assignment satisfies the claim. */
  public static final String QUESTION_SATISFIABLE = "satisfiable";

  /**
   * Creates a verdict.
   *
   * @param kind what was verified, never {@code null}
   * @param elementId the element's FQN, when it has one
   * @param element the element as named, never {@code null}
   * @param holds the answer
   * @param condition the failing condition, when named
   * @param instanceId the object, when there is one
   * @param instanceTypeId that object's type, when there is one
   * @param error the failure, when evaluation failed
   * @param failureReason the kind of failure, never {@code null}
   * @param requirementId the requirement asserted satisfied, when named
   * @param instancePath the path to the object, when validating
   * @param question the question answered, when reported
   * @param status the answer's status, when reported
   * @param witness the assignment witnessing the answer, never {@code null}
   * @param standing the standing, never {@code null}
   */
  public Verdict {
    Objects.requireNonNull(kind, "kind");
    Objects.requireNonNull(elementId, "elementId");
    Objects.requireNonNull(element, "element");
    Objects.requireNonNull(condition, "condition");
    Objects.requireNonNull(instanceId, "instanceId");
    Objects.requireNonNull(instanceTypeId, "instanceTypeId");
    Objects.requireNonNull(error, "error");
    Objects.requireNonNull(failureReason, "failureReason");
    Objects.requireNonNull(requirementId, "requirementId");
    Objects.requireNonNull(instancePath, "instancePath");
    Objects.requireNonNull(question, "question");
    Objects.requireNonNull(status, "status");
    witness = List.copyOf(Objects.requireNonNull(witness, "witness"));
    Objects.requireNonNull(standing, "standing");
  }

  /**
   * Whether the model answered at all: evaluation succeeded and {@link #holds()} is its answer.
   *
   * @return {@code true} when no error was reported
   */
  public boolean decided() {
    return error.isEmpty();
  }

  /**
   * Whether the model answered false: the condition was evaluated and did not hold.
   *
   * @return {@code true} for a decided verdict that does not hold
   */
  public boolean violated() {
    return decided() && !holds;
  }
}
