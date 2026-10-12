package org.openmbee.opensysml;

import java.util.Collections;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Objects;

/**
 * What a verification asks and the values it asks it over: an evaluation of the declared or held
 * values (the default), or a {@code holds} or {@code satisfiable} question the service's solvers
 * answer — a question other than {@link #QUESTION_EVALUATE} needs {@code verification_questions}.
 * The arguments bind the constraint's or requirement's {@code in} parameters, positionally in
 * declaration order or by name; any binding needs {@code verification_arguments} and only an
 * evaluation accepts one, since a solver question decides those values itself.
 *
 * @param question the question, as the service spells it
 * @param arguments values binding the {@code in} parameters in declaration order
 * @param namedArguments values binding {@code in} parameters by name
 */
public record VerifyOptions(
    String question, List<Value> arguments, Map<String, Value> namedArguments) {

  /** The evaluation a verification asks unless a question names another. */
  public static final String QUESTION_EVALUATE = "evaluate";

  /** The question whether the claim holds for every assignment the free features can take. */
  public static final String QUESTION_HOLDS = "holds";

  /** The question whether any free assignment satisfies the claim. */
  public static final String QUESTION_SATISFIABLE = "satisfiable";

  /**
   * Creates options, copying the argument collections.
   *
   * @param question the question
   * @param arguments the positional arguments
   * @param namedArguments the named arguments
   */
  public VerifyOptions {
    Objects.requireNonNull(question, "question");
    Objects.requireNonNull(arguments, "arguments");
    Objects.requireNonNull(namedArguments, "namedArguments");
    arguments = List.copyOf(arguments);
    namedArguments = Collections.unmodifiableMap(new LinkedHashMap<>(namedArguments));
  }

  /**
   * Creates options asking a question with no arguments.
   *
   * @param question the question
   */
  public VerifyOptions(String question) {
    this(question, List.of(), Map.of());
  }

  /**
   * The evaluation, with no arguments.
   *
   * @return the default options
   */
  public static VerifyOptions defaults() {
    return new VerifyOptions(QUESTION_EVALUATE);
  }

  /**
   * Creates options asking a question.
   *
   * @param question the question, as the service spells it
   * @return the options
   */
  public static VerifyOptions asking(String question) {
    return new VerifyOptions(question);
  }

  /**
   * The same options with these positional arguments.
   *
   * @param arguments the arguments, in parameter order
   * @return the options
   */
  public VerifyOptions withArguments(List<Value> arguments) {
    return new VerifyOptions(question, arguments, namedArguments);
  }

  /**
   * The same options with these positional arguments.
   *
   * @param arguments the arguments, in parameter order
   * @return the options
   */
  public VerifyOptions withArguments(Value... arguments) {
    return withArguments(List.of(arguments));
  }

  /**
   * The same options with these named arguments.
   *
   * @param namedArguments the arguments by parameter name
   * @return the options
   */
  public VerifyOptions withNamedArguments(Map<String, Value> namedArguments) {
    return new VerifyOptions(question, arguments, namedArguments);
  }

  /**
   * Whether any argument is bound.
   *
   * @return true when a positional or named argument is present
   */
  public boolean bindsArguments() {
    return !arguments.isEmpty() || !namedArguments.isEmpty();
  }
}
