package org.openmbee.opensysml;

import java.util.List;
import java.util.Objects;
import java.util.Optional;

/**
 * How {@link Model#checkAction} and {@link Model#checkStateMachine} search a behavior: the
 * properties to hold at every stable state, the features allowed to diverge, what the symbolic
 * engine ranges over and assumes, the bounds, and the engine asked.
 *
 * @param properties constraints or requirements evaluated at every stable state
 * @param diverge features whose final value the schedule may decide; {@code this.<name>} names
 *     the performer's
 * @param freeInputs inputs the symbolic engine ranges over
 * @param assume constraints or requirements the symbolic engine assumes
 * @param witness whether to carry each witness's file in the answer
 * @param depth the bound on moves per schedule; 0 is the engine's default
 * @param states the bound on states visited; 0 is the engine's default
 * @param unroll the bound on loop unrollings; 0 is the engine's default
 * @param timeout the seconds the search may take; 0 is none
 * @param engine the engine asked: {@code check} (the default), {@code smt} or one {@link
 *     Connection#listEngines()} names
 * @param performer the part or usage whose object performs the behavior
 */
public record CheckOptions(
    List<String> properties,
    List<String> diverge,
    List<String> freeInputs,
    List<String> assume,
    boolean witness,
    int depth,
    int states,
    int unroll,
    double timeout,
    Optional<String> engine,
    Optional<String> performer) {

  /** Creates check options; every list is copied. */
  public CheckOptions {
    properties = List.copyOf(properties);
    diverge = List.copyOf(diverge);
    freeInputs = List.copyOf(freeInputs);
    assume = List.copyOf(assume);
    Objects.requireNonNull(engine, "engine");
    Objects.requireNonNull(performer, "performer");
  }

  /** No properties, no bounds, the default engine, no performer. */
  public static CheckOptions defaults() {
    return new CheckOptions(
        List.of(), List.of(), List.of(), List.of(), false, 0, 0, 0, 0.0,
        Optional.empty(), Optional.empty());
  }

  /** These options with the properties to hold at every stable state. */
  public CheckOptions withProperties(String... names) {
    return new CheckOptions(List.of(names), diverge, freeInputs, assume, witness, depth, states,
        unroll, timeout, engine, performer);
  }

  /** These options with the features whose final value the schedule may decide. */
  public CheckOptions withDiverge(String... features) {
    return new CheckOptions(properties, List.of(features), freeInputs, assume, witness, depth,
        states, unroll, timeout, engine, performer);
  }

  /** These options with the inputs the symbolic engine ranges over. */
  public CheckOptions withFreeInputs(String... names) {
    return new CheckOptions(properties, diverge, List.of(names), assume, witness, depth, states,
        unroll, timeout, engine, performer);
  }

  /** These options with the constraints or requirements the symbolic engine assumes. */
  public CheckOptions withAssume(String... names) {
    return new CheckOptions(properties, diverge, freeInputs, List.of(names), witness, depth,
        states, unroll, timeout, engine, performer);
  }

  /** These options carrying each witness's file in the answer. */
  public CheckOptions withWitness() {
    return new CheckOptions(properties, diverge, freeInputs, assume, true, depth, states, unroll,
        timeout, engine, performer);
  }

  /** These options bounded in moves per schedule, states visited, loop unrollings and seconds. */
  public CheckOptions withBounds(int depth, int states, int unroll, double timeout) {
    return new CheckOptions(properties, diverge, freeInputs, assume, witness, depth, states,
        unroll, timeout, engine, performer);
  }

  /** These options asking the named engine. */
  public CheckOptions withEngine(String engine) {
    return new CheckOptions(properties, diverge, freeInputs, assume, witness, depth, states,
        unroll, timeout, Optional.of(engine), performer);
  }

  /** These options performed by the named part or usage's object. */
  public CheckOptions withPerformer(String performer) {
    return new CheckOptions(properties, diverge, freeInputs, assume, witness, depth, states,
        unroll, timeout, engine, Optional.of(performer));
  }
}
