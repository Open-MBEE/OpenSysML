package org.openmbee.opensysml;

import java.util.List;
import java.util.Objects;
import java.util.Optional;

/**
 * One application of a calc an analysis case declares, made while the case ran. For a trade study
 * it is the evaluation function scoring one alternative. Reported by a service advertising the
 * {@code case_evaluations} capability.
 *
 * @param functionId FQN of the calc applied
 * @param arguments what it was applied to, in parameter order; an alternative is an {@link
 *     Value.InstanceReference} the {@link Analysis} resolves
 * @param result what it computed, absent when {@code error} says why it computed nothing
 * @param error why the application failed, absent when it computed a value
 * @param selected whether this is the evaluation whose argument {@code selectOne} picked and the
 *     case returned: the alternative a trade study selected
 * @param tied whether this evaluation computed what the selected one did without being it
 */
public record CaseEvaluation(
    String functionId,
    List<Value> arguments,
    Optional<Value> result,
    Optional<String> error,
    boolean selected,
    boolean tied) {

  /**
   * Creates an evaluation, copying its arguments.
   *
   * @param functionId the calc's FQN, never {@code null}
   * @param arguments the arguments
   * @param result the result, when there is one
   * @param error the failure, when there is one
   * @param selected whether it was selected
   * @param tied whether it tied the selected one
   */
  public CaseEvaluation {
    Objects.requireNonNull(functionId, "functionId");
    arguments = List.copyOf(arguments);
    Objects.requireNonNull(result, "result");
    Objects.requireNonNull(error, "error");
  }
}
