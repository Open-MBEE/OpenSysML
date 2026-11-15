package org.openmbee.opensysml;

import java.util.List;
import java.util.Objects;

/**
 * The states entered and transitions taken across every run of an exploration or every schedule
 * of a check, not only the witness of each outcome: what {@link Exploration#coverage()} and
 * {@link CheckResult.Search#coverage()} answer.
 *
 * @param states the states entered, in the service's order
 * @param transitions the transitions taken, in the service's order
 */
public record Coverage(List<Entry> states, List<Entry> transitions) {

  /** The coverage of a service that reports none. */
  public static final Coverage NONE = new Coverage(List.of(), List.of());

  /** Creates a coverage, copying its lists. */
  public Coverage {
    states = List.copyOf(states);
    transitions = List.copyOf(transitions);
  }

  /**
   * One state entered or transition taken, and how many runs did.
   *
   * @param path the state or transition under the instance running the machine, {@code
   *     "<object>.<machine>: <state>"} or {@code "<object>.<machine>: <from> -> <to>"}
   * @param declaration the same under its declaration, {@code "<machine>::<state>"} or {@code
   *     "<machine>::<from> -> <to>"}
   * @param runs how many runs (schedules, for a check) entered the state or took the transition
   */
  public record Entry(String path, String declaration, int runs) {
    /** Creates an entry; the path and declaration are required. */
    public Entry {
      Objects.requireNonNull(path, "path");
      Objects.requireNonNull(declaration, "declaration");
    }
  }
}
