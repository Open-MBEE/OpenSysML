package org.openmbee.opensysml;

import java.io.Serializable;
import java.util.List;
import java.util.Objects;

/**
 * Why a document query could not be run: the structured record the service attaches to the status
 * of a failed query, mirroring the {@code QueryOutcome} wire message and the CLI's {@code -json}
 * outcome record.
 *
 * <p>A {@linkplain #isPathError() path error} means the model has no answer for a query step: each
 * {@link Failure} names the step, what it expected and the {@link Frontier frontier} where the path
 * stopped, with the candidates there. A {@linkplain #isModelError() model error} means the model
 * breaks a constraint the query relies on: each {@link Finding} names the element, the constraint
 * and what was observed, and {@link #pathFailures()} carries any path failures from the same step.
 *
 * @param kind the outcome kind, {@link #PATH_ERROR} or {@link #MODEL_ERROR}
 * @param step the label of the step the outcome is reported at, such as {@code Feature(mass)}
 * @param failures a path error's failures, one per failing element; empty for a model error
 * @param findings a model error's findings; empty for a path error
 * @param pathFailures a model error's path failures from the same step; empty for a path error
 */
public record QueryOutcome(
    String kind,
    String step,
    List<Failure> failures,
    List<Finding> findings,
    List<Failure> pathFailures)
    implements Serializable {

  /** The kind of an outcome whose path has no answer in the model. */
  public static final String PATH_ERROR = "pathError";

  /** The kind of an outcome whose model breaks a constraint the query relies on. */
  public static final String MODEL_ERROR = "modelError";

  /**
   * Creates an outcome, copying its collections.
   *
   * @param kind the outcome kind
   * @param step the step label
   * @param failures the path failures
   * @param findings the model findings
   * @param pathFailures the path failures a model error carries
   */
  public QueryOutcome {
    Objects.requireNonNull(kind, "kind");
    Objects.requireNonNull(step, "step");
    failures = List.copyOf(failures);
    findings = List.copyOf(findings);
    pathFailures = List.copyOf(pathFailures);
  }

  /**
   * Whether the outcome is a path error.
   *
   * @return {@code true} when {@link #kind()} is {@link #PATH_ERROR}
   */
  public boolean isPathError() {
    return PATH_ERROR.equals(kind);
  }

  /**
   * Whether the outcome is a model error.
   *
   * @return {@code true} when {@link #kind()} is {@link #MODEL_ERROR}
   */
  public boolean isModelError() {
    return MODEL_ERROR.equals(kind);
  }

  /**
   * One element the query path stopped at.
   *
   * @param step the label of the failing step, such as {@code Feature(mass)}
   * @param expected what the step expected there, such as {@code a feature named mass on the
   *     element}
   * @param frontier where the path stopped and what is there
   */
  public record Failure(String step, String expected, Frontier frontier) implements Serializable {

    /**
     * Creates a failure.
     *
     * @param step the step label
     * @param expected the expectation
     * @param frontier the frontier
     */
    public Failure {
      Objects.requireNonNull(step, "step");
      Objects.requireNonNull(expected, "expected");
      Objects.requireNonNull(frontier, "frontier");
    }
  }

  /**
   * Where a query path stopped: the last element it resolved and the candidates there. Which
   * candidate list is filled depends on what the failing step expected: members when it expected a
   * name, features when it expected a feature, relationship kinds when it expected a kind.
   *
   * @param at the qualified name of the element the path stopped at, or {@code ""} when nothing on
   *     the path resolved
   * @param members the element's members, when a name was expected
   * @param features the element's features, when a feature was expected
   * @param kinds the relationship kinds the element has, when a kind was expected
   * @param nearest the member whose spelling is closest to the one the step named, or {@code ""}
   *     when no name was expected or none is close
   */
  public record Frontier(
      String at, List<Member> members, List<Feature> features, List<String> kinds, String nearest)
      implements Serializable {

    /**
     * Creates a frontier, copying its collections.
     *
     * @param at the element the path stopped at
     * @param members the candidate members
     * @param features the candidate features
     * @param kinds the candidate relationship kinds
     * @param nearest the closest spelling
     */
    public Frontier {
      Objects.requireNonNull(at, "at");
      members = List.copyOf(members);
      features = List.copyOf(features);
      kinds = List.copyOf(kinds);
      Objects.requireNonNull(nearest, "nearest");
    }

    /**
     * A member of the element a path stopped at.
     *
     * @param name the member's name
     * @param metaclass the member's metaclass, such as {@code PartUsage}
     */
    public record Member(String name, String metaclass) implements Serializable {

      /**
       * Creates a member.
       *
       * @param name the name
       * @param metaclass the metaclass
       */
      public Member {
        Objects.requireNonNull(name, "name");
        Objects.requireNonNull(metaclass, "metaclass");
      }
    }

    /**
     * A feature of the element a path stopped at.
     *
     * @param name the feature's name
     * @param type the feature's declared type, or {@code ""} when it has none
     */
    public record Feature(String name, String type) implements Serializable {

      /**
       * Creates a feature.
       *
       * @param name the name
       * @param type the type
       */
      public Feature {
        Objects.requireNonNull(name, "name");
        Objects.requireNonNull(type, "type");
      }
    }
  }

  /**
   * One constraint the model breaks.
   *
   * @param step the label of the step that found it
   * @param element the qualified name of the element that breaks the constraint
   * @param constraint the constraint's name, such as {@code ambiguousName}
   * @param observed what was observed, such as {@code Duplicates::repeated (declared 2 times)}
   */
  public record Finding(String step, String element, String constraint, String observed)
      implements Serializable {

    /**
     * Creates a finding.
     *
     * @param step the step label
     * @param element the element
     * @param constraint the constraint name
     * @param observed the observation
     */
    public Finding {
      Objects.requireNonNull(step, "step");
      Objects.requireNonNull(element, "element");
      Objects.requireNonNull(constraint, "constraint");
      Objects.requireNonNull(observed, "observed");
    }
  }
}
