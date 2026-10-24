package org.openmbee.opensysml;

import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.Optional;

/**
 * What {@code CheckBehavior} answered: the verdict over every schedule of a behavior, each
 * violation, divergence and final outcome witnessed by a schedule that replays it. This is the
 * report {@code sysml -check-property} prints, as data.
 *
 * @param verdict {@code holds}, {@code fails} or {@code undecided}
 * @param lines the report as the CLI prints it
 * @param search the explicit-state search's statistics; empty for a symbolic answer
 * @param violations each property, deadlock or failure witnessed
 * @param divergences each feature whose final value the schedule decided
 * @param finals each final outcome of an exhaustive search
 * @param claim the symbolic engine's claim
 * @param freeInputs the inputs it ranged over
 * @param assumptions what it assumed
 * @param witness its witness
 * @param contrast its counter-witness
 * @param standing the engine, its strength and the bounds it ran under
 * @param reason why the verdict is undecided, when it is
 */
public record CheckResult(
    String verdict,
    List<String> lines,
    Optional<Search> search,
    List<Violation> violations,
    List<Divergence> divergences,
    List<Final> finals,
    String claim,
    List<String> freeInputs,
    List<String> assumptions,
    Optional<Witness> witness,
    Optional<Witness> contrast,
    Standing standing,
    String reason) {

  private static final String WITNESS_NAME = "witness";

  /** Creates a check result; every list is copied. */
  public CheckResult {
    Objects.requireNonNull(verdict, "verdict");
    lines = List.copyOf(lines);
    Objects.requireNonNull(search, "search");
    violations = List.copyOf(violations);
    divergences = List.copyOf(divergences);
    finals = List.copyOf(finals);
    Objects.requireNonNull(claim, "claim");
    freeInputs = List.copyOf(freeInputs);
    assumptions = List.copyOf(assumptions);
    Objects.requireNonNull(witness, WITNESS_NAME);
    Objects.requireNonNull(contrast, "contrast");
    Objects.requireNonNull(standing, "standing");
    Objects.requireNonNull(reason, "reason");
  }

  /** Whether every schedule kept every property. */
  public boolean holds() {
    return verdict.equals("holds");
  }

  @Override
  public String toString() {
    return lines.isEmpty() ? verdict : String.join("\n", lines);
  }

  /**
   * One schedule replayed: pass {@link #schedule()} as a run's schedule to follow it.
   *
   * @param schedule the replay schedule policy
   * @param draws the draws the schedule took
   * @param choices the choices it made at each choice point
   * @param inputs the inputs the symbolic engine chose, as {@code name = value}
   * @param fileName the witness file's name when one was asked for, else empty
   * @param fileContent that file's content
   */
  public record Witness(
      String schedule,
      List<String> draws,
      List<String> choices,
      List<String> inputs,
      String fileName,
      String fileContent) {
    /** Creates a witness; every list is copied. */
    public Witness {
      Objects.requireNonNull(schedule, "schedule");
      draws = List.copyOf(draws);
      choices = List.copyOf(choices);
      inputs = List.copyOf(inputs);
      Objects.requireNonNull(fileName, "fileName");
      Objects.requireNonNull(fileContent, "fileContent");
    }
  }

  /**
   * One violation witnessed.
   *
   * @param kind {@code property}, {@code deadlock} or {@code failure}
   * @param property the property false at the state, for a property violation
   * @param error the failure's message, for a failure
   * @param depth moves from the start to the violating state
   * @param probability the schedule's probability under a weighted scheduler, else 0
   * @param witness the schedule reaching it
   * @param text the violation as the report prints it
   */
  public record Violation(
      String kind,
      String property,
      String error,
      int depth,
      double probability,
      Optional<Witness> witness,
      String text) {
    /** Creates a violation. */
    public Violation {
      Objects.requireNonNull(kind, "kind");
      Objects.requireNonNull(property, "property");
      Objects.requireNonNull(error, "error");
      Objects.requireNonNull(witness, WITNESS_NAME);
      Objects.requireNonNull(text, "text");
    }
  }

  /**
   * One final value a feature reaches and the schedule reaching it.
   *
   * @param value the value as the report prints it
   * @param witness the schedule reaching it
   */
  public record DivergentValue(String value, Optional<Witness> witness) {
    /** Creates a divergent value. */
    public DivergentValue {
      Objects.requireNonNull(value, "value");
      Objects.requireNonNull(witness, WITNESS_NAME);
    }
  }

  /**
   * A feature whose final value the schedule decides.
   *
   * @param feature the feature as asked for
   * @param values each final value with a schedule reaching it
   */
  public record Divergence(String feature, List<DivergentValue> values) {
    /** Creates a divergence; the values are copied. */
    public Divergence {
      Objects.requireNonNull(feature, "feature");
      values = List.copyOf(values);
    }
  }

  /**
   * One final outcome of an exhaustive search.
   *
   * @param outcome the outcome as the report prints it
   * @param values each tracked feature's final value
   * @param witness a schedule reaching it
   */
  public record Final(String outcome, Map<String, String> values, Optional<Witness> witness) {
    /** Creates a final outcome; the values are copied. */
    public Final {
      Objects.requireNonNull(outcome, "outcome");
      values = Map.copyOf(values);
      Objects.requireNonNull(witness, WITNESS_NAME);
    }
  }

  /**
   * The explicit-state search's statistics.
   *
   * @param states the states visited
   * @param moves the moves taken
   * @param maxDepth the deepest schedule
   * @param boundsHit the bounds the search stopped at: {@code depth}, {@code states}, {@code
   *     timeout}
   * @param horizon the clock horizon, meaningful when {@code horizonSet}
   * @param horizonSet whether a horizon applied
   * @param massBounded whether the probability mass was bounded
   * @param notes what the search notes about its scope
   * @param scope the scope the search ran over
   * @param status the search's status as the report prints it
   */
  public record Search(
      int states,
      int moves,
      int maxDepth,
      List<String> boundsHit,
      double horizon,
      boolean horizonSet,
      boolean massBounded,
      List<String> notes,
      List<String> scope,
      String status) {
    /** Creates search statistics; every list is copied. */
    public Search {
      boundsHit = List.copyOf(boundsHit);
      notes = List.copyOf(notes);
      scope = List.copyOf(scope);
      Objects.requireNonNull(status, "status");
    }
  }
}
