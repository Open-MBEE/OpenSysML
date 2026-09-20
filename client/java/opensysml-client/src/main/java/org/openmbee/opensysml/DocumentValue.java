package org.openmbee.opensysml;

import java.util.List;
import java.util.Objects;
import java.util.Optional;

/**
 * One typed document-query value: what a {@code bindings} value of {@link
 * Model#runDocumentQuery(String, java.util.Map)} carries, and what a {@link DocumentRow}'s element
 * and cells answer.
 *
 * <p>An element is bound by qualified name ({@link ElementRef}) and an object the service holds by
 * id or by path ({@link ObjectRef}); a row a {@code Verdicts}, {@code States} or {@code Events}
 * query answered is a {@link DocumentVerdict}, {@link DocumentState} or {@link DocumentEvent}, and
 * an unbounded multiplicity is {@link InfinityValue}. Binding a verdict, state, event or infinity
 * is sent as-is: the service refuses it, rather than the client deciding which kinds bind.
 */
public sealed interface DocumentValue {

  /**
   * A model element, named by qualified name.
   *
   * @param id the element's qualified name; empty for an anonymous element a query answered
   * @param type the element's metamodel type ({@code "PartUsage"}, …); reported when answered,
   *     carried when bound so an anonymous element can be named by type alone
   */
  record ElementRef(String id, String type) implements DocumentValue {

    /**
     * Creates an element reference.
     *
     * @param id the qualified name, never {@code null}
     * @param type the metamodel type, empty rather than {@code null} when unreported
     */
    public ElementRef {
      Objects.requireNonNull(id, "id");
      Objects.requireNonNull(type, "type");
    }
  }

  /**
   * An object the service holds for the model, created by {@link Model#instantiate(String)}.
   *
   * <p>Bound, it names the object by {@code path} when set and by {@code id} otherwise; one setting
   * both must name one object by both. Answered, it carries all three fields.
   *
   * @param id the object's id, as the instantiation answered it
   * @param path the object by the label a session reaches it under: the qualified name it was
   *     instantiated as ({@code "Garage::car"}), its id ({@code "#2"}), or a path through feature
   *     values of either ({@code "Garage::car.wheels[2]"}; indexes count from 1)
   * @param element the usage the object is held under, its definition or usage; reported when
   *     answered, ignored when bound
   */
  record ObjectRef(long id, String path, Optional<ElementRef> element) implements DocumentValue {

    /**
     * Creates an object reference.
     *
     * @param id the object's id
     * @param path the object's path, empty rather than {@code null} when unnamed
     * @param element the holding usage, when reported
     */
    public ObjectRef {
      Objects.requireNonNull(path, "path");
      Objects.requireNonNull(element, "element");
    }
  }

  /**
   * A string.
   *
   * @param value the text
   */
  record StringValue(String value) implements DocumentValue {

    /**
     * Creates the value.
     *
     * @param value the text, never {@code null}
     */
    public StringValue {
      Objects.requireNonNull(value, "value");
    }
  }

  /**
   * An integer.
   *
   * @param value the integer
   */
  record IntegerValue(long value) implements DocumentValue {}

  /**
   * A real.
   *
   * @param value the real
   */
  record RealValue(double value) implements DocumentValue {}

  /**
   * A boolean.
   *
   * @param value the boolean
   */
  record BooleanValue(boolean value) implements DocumentValue {}

  /** The unbounded value {@code *}: answered, never bound. */
  record InfinityValue() implements DocumentValue {}

  /**
   * A magnitude in a unit.
   *
   * @param quantity the quantity
   */
  record QuantityValue(Quantity quantity) implements DocumentValue {

    /**
     * Creates the value.
     *
     * @param quantity the quantity, never {@code null}
     */
    public QuantityValue {
      Objects.requireNonNull(quantity, "quantity");
    }
  }

  /**
   * A row a {@code Verdicts} query answered: an assertion checked on one object. Answered only;
   * binding one is refused.
   *
   * @param assertion the constraint, requirement, satisfy usage or verification case checked; its
   *     id is empty when the assertion is anonymous
   * @param kind {@code "constraint"}, {@code "requirement"}, {@code "satisfaction"} or {@code
   *     "verification"}
   * @param text the assertion as written ({@code "assert constraint massKnown"})
   * @param path the object checked, named from the element the query was bound to ({@code
   *     "Garage::car.wheels[2]"})
   * @param status {@code "holds"}, {@code "violated"} or {@code "undecided"}
   * @param condition the condition that evaluated to false, as written; empty otherwise
   * @param reason why the assertion is violated or undecided; empty when it holds
   * @param verification the verdict kinds ({@code "pass"}, {@code "fail"}, …) of the verification
   *     cases verifying the requirement the row is about; a verification row's own kind
   */
  record DocumentVerdict(
      ElementRef assertion,
      String kind,
      String text,
      String path,
      String status,
      String condition,
      String reason,
      List<String> verification)
      implements DocumentValue {

    /**
     * Creates a verdict row, copying its verification kinds.
     *
     * @param assertion the assertion checked, never {@code null}
     * @param kind the verdict's kind, never {@code null}
     * @param text the assertion as written, never {@code null}
     * @param path the object checked, never {@code null}
     * @param status how it came out, never {@code null}
     * @param condition the failed condition, never {@code null}
     * @param reason why it failed, never {@code null}
     * @param verification the verification verdict kinds
     */
    public DocumentVerdict {
      Objects.requireNonNull(assertion, "assertion");
      Objects.requireNonNull(kind, "kind");
      Objects.requireNonNull(text, "text");
      Objects.requireNonNull(path, "path");
      Objects.requireNonNull(status, "status");
      Objects.requireNonNull(condition, "condition");
      Objects.requireNonNull(reason, "reason");
      verification = List.copyOf(verification);
    }
  }

  /**
   * A row a {@code States} query answered: one active leaf state of one object. Answered only;
   * binding one is refused.
   *
   * @param object the object whose state machine the row reads
   * @param machine the exhibited state usage ({@code "lp"}), or the state def's name
   * @param name the active leaf state's own name ({@code "dim"})
   * @param path the leaf's path from the machine's top level ({@code "on.dim"})
   * @param state the leaf's declaration, absent when the machine declares no element for it
   * @param region the orthogonal region declaring the leaf ({@code "light"}); empty when the leaf
   *     is not in one
   * @param enclosing the active composite states around the leaf, outermost first
   */
  record DocumentState(
      ObjectRef object,
      String machine,
      String name,
      String path,
      Optional<ElementRef> state,
      String region,
      List<String> enclosing)
      implements DocumentValue {

    /**
     * Creates a state row, copying its enclosing states.
     *
     * @param object the object, never {@code null}
     * @param machine the machine's name, never {@code null}
     * @param name the leaf's name, never {@code null}
     * @param path the leaf's path, never {@code null}
     * @param state the leaf's declaration, when the model names one
     * @param region the orthogonal region, never {@code null}
     * @param enclosing the enclosing states
     */
    public DocumentState {
      Objects.requireNonNull(object, "object");
      Objects.requireNonNull(machine, "machine");
      Objects.requireNonNull(name, "name");
      Objects.requireNonNull(path, "path");
      Objects.requireNonNull(state, "state");
      Objects.requireNonNull(region, "region");
      enclosing = List.copyOf(enclosing);
    }
  }

  /**
   * A row an {@code Events} query answered: one record of a session's trace. Answered only;
   * binding one is refused.
   *
   * @param kind {@code "accept"}, {@code "send"}, {@code "transition"}, {@code "entry"}, {@code
   *     "exit"}, {@code "do"}, {@code "choice"} or {@code "guard"}
   * @param time the instant the record was written at, in the runtime clock's unit — a {@link
   *     QuantityValue} when the clock carries a unit, a plain number otherwise
   * @param text the record as the trace prints it
   * @param object the object the record is about; absent for a record of the run as a whole
   * @param machine the state machine the record is about, as {@link DocumentState#machine()} names
   *     it
   * @param state the state entered, exited or run (entry, exit and do records)
   * @param from the transition's source state
   * @param to the transition's target state
   * @param target the object a send was delivered to; absent for every other kind
   * @param event the accepted or sent event's type name
   * @param payload the accept's payload, one {@code name = value} text per attribute
   * @param alternatives what a choice drew from, in order
   * @param taken the alternative the choice took
   */
  record DocumentEvent(
      String kind,
      DocumentValue time,
      String text,
      Optional<ObjectRef> object,
      String machine,
      String state,
      String from,
      String to,
      Optional<ObjectRef> target,
      String event,
      List<String> payload,
      List<String> alternatives,
      String taken)
      implements DocumentValue {

    /**
     * Creates an event row, copying its lists.
     *
     * @param kind the record's kind, never {@code null}
     * @param time the record's instant, never {@code null}
     * @param text the record as printed, never {@code null}
     * @param object the object the record is about, when there is one
     * @param machine the machine's name, never {@code null}
     * @param state the state, never {@code null}
     * @param from the source state, never {@code null}
     * @param to the target state, never {@code null}
     * @param target the addressed object, when there is one
     * @param event the event's type name, never {@code null}
     * @param payload the payload entries
     * @param alternatives the alternatives
     * @param taken the one taken, never {@code null}
     */
    public DocumentEvent {
      Objects.requireNonNull(kind, "kind");
      Objects.requireNonNull(time, "time");
      Objects.requireNonNull(text, "text");
      Objects.requireNonNull(object, "object");
      Objects.requireNonNull(machine, "machine");
      Objects.requireNonNull(state, "state");
      Objects.requireNonNull(from, "from");
      Objects.requireNonNull(to, "to");
      Objects.requireNonNull(target, "target");
      Objects.requireNonNull(event, "event");
      payload = List.copyOf(payload);
      alternatives = List.copyOf(alternatives);
      Objects.requireNonNull(taken, "taken");
    }
  }
}
