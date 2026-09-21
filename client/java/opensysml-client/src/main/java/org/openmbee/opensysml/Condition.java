package org.openmbee.opensysml;

import java.util.List;
import java.util.Objects;

/**
 * A {@link Query} filter: one comparison of a property, or several conditions combined. Build one
 * with {@link #equalTo}, {@link #greater}, {@link #less}, {@link #all} or {@link #any}, and negate it
 * with {@link #negated()}.
 */
public sealed interface Condition {

  /**
   * A condition matching an element whose property is any of the values given. The property names
   * one of the query properties the API reference documents ({@code "@type"}, {@code "name"},
   * {@code "qualifiedName"}, ...); an unknown one fails the call.
   *
   * @param property the property
   * @param values the values it may equal
   * @return the comparison
   */
  static Comparison equalTo(String property, List<String> values) {
    return new Comparison(property, Comparison.Operator.EQUAL, values, false);
  }

  /**
   * A condition matching an element whose property is greater than the value.
   *
   * @param property the property
   * @param value the value
   * @return the comparison
   */
  static Comparison greater(String property, String value) {
    return new Comparison(property, Comparison.Operator.GREATER, List.of(value), false);
  }

  /**
   * A condition matching an element whose property is less than the value.
   *
   * @param property the property
   * @param value the value
   * @return the comparison
   */
  static Comparison less(String property, String value) {
    return new Comparison(property, Comparison.Operator.LESS, List.of(value), false);
  }

  /**
   * A condition matching an element every condition matches. An empty list fails the call: it has
   * no defensible verdict.
   *
   * @param conditions the conditions
   * @return the combination
   */
  static Combination all(List<Condition> conditions) {
    return new Combination(Combination.Operator.AND, conditions);
  }

  /**
   * A condition matching an element at least one condition matches. An empty list fails the call.
   *
   * @param conditions the conditions
   * @return the combination
   */
  static Combination any(List<Condition> conditions) {
    return new Combination(Combination.Operator.OR, conditions);
  }

  /**
   * The condition matching what this one does not. A comparison negates its verdict; a
   * combination, which the wire has no negation for, negates each operand and swaps {@code all}
   * for {@code any}, which says the same thing.
   *
   * @return the negation
   */
  Condition negated();

  /**
   * One comparison of a property against values.
   *
   * @param property the property compared
   * @param operator how it is compared
   * @param values what it is compared against: any of them for {@link Operator#EQUAL}, the one
   *     value for the others
   * @param inverse whether the comparison's verdict is negated
   */
  record Comparison(String property, Operator operator, List<String> values, boolean inverse)
      implements Condition {

    /** How a property is compared. */
    public enum Operator {
      /** The property equals one of the values. */
      EQUAL,
      /** The property is greater than the value. */
      GREATER,
      /** The property is less than the value. */
      LESS
    }

    /**
     * Creates a comparison, copying its values.
     *
     * @param property the property, never {@code null}
     * @param operator the operator, never {@code null}
     * @param values the values
     * @param inverse whether it is negated
     */
    public Comparison {
      Objects.requireNonNull(property, "property");
      Objects.requireNonNull(operator, "operator");
      values = List.copyOf(values);
    }

    @Override
    public Comparison negated() {
      return new Comparison(property, operator, values, !inverse);
    }
  }

  /**
   * Several conditions combined.
   *
   * @param operator how they combine
   * @param conditions the conditions combined
   */
  record Combination(Operator operator, List<Condition> conditions) implements Condition {

    /** How conditions combine. */
    public enum Operator {
      /** Every condition must match. */
      AND,
      /** At least one condition must match. */
      OR
    }

    /**
     * Creates a combination, copying its conditions.
     *
     * @param operator the operator, never {@code null}
     * @param conditions the conditions
     */
    public Combination {
      Objects.requireNonNull(operator, "operator");
      conditions = List.copyOf(conditions);
    }

    @Override
    public Combination negated() {
      Operator swapped = operator == Operator.AND ? Operator.OR : Operator.AND;
      return new Combination(swapped, conditions.stream().map(Condition::negated).toList());
    }
  }
}
