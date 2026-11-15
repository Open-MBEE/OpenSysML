package org.openmbee.opensysml;

import java.util.ArrayList;
import java.util.List;
import java.util.Objects;

/**
 * Every distinct outcome a behavior reaches under some order of its choice points, and how the
 * search for them ended: what {@link Model#exploreAction(String)} and its siblings answer.
 *
 * @param outcomes the outcomes reached, in the order first reached
 * @param complete whether every order within the budget ran
 * @param runs how many runs were made
 * @param budgetsHit the budgets that ended the search ({@code "runs"}, {@code "depth"}), empty when
 *     it completed
 * @param runsBudget the most runs the search would make
 * @param depthBudget the most choice points one run would resolve
 * @param failedLinearizations the runs whose outcomes carry runtime errors
 * @param probabilitiesLowerBound whether weighted probabilities are lower bounds: a budget kept
 *     some orders unexplored
 * @param coverage the states entered and transitions taken across every run, not only each
 *     outcome's witness
 */
public record Exploration(
    List<Outcome> outcomes,
    boolean complete,
    int runs,
    List<String> budgetsHit,
    int runsBudget,
    int depthBudget,
    int failedLinearizations,
    boolean probabilitiesLowerBound,
    Coverage coverage) {

  /**
   * Creates an exploration, copying its collections.
   *
   * @param outcomes the outcomes
   * @param complete whether the search completed
   * @param runs the runs made
   * @param budgetsHit the budgets that ended it
   * @param runsBudget the run budget
   * @param depthBudget the depth budget
   * @param failedLinearizations the failed runs
   * @param probabilitiesLowerBound whether the probabilities are lower bounds
   * @param coverage the coverage of every run
   */
  public Exploration {
    outcomes = List.copyOf(outcomes);
    budgetsHit = List.copyOf(budgetsHit);
    Objects.requireNonNull(coverage, "coverage");
  }

  /**
   * How the search ended, as the {@code sysml} command renders it: {@code "complete (6 runs)"} or
   * {@code "incomplete: runs budget 100 hit after 100 runs"}.
   *
   * @return the status line
   */
  public String status() {
    if (complete) {
      return "complete (" + runs + " runs)";
    }
    List<String> named = new ArrayList<>(budgetsHit.size());
    for (String budget : budgetsHit) {
      int limit = budget.equals("depth") ? depthBudget : runsBudget;
      named.add(budget + " budget " + limit);
    }
    return "incomplete: "
        + String.join(" and ", named)
        + " hit after "
        + runs
        + " runs"
        + (probabilitiesLowerBound ? "; probabilities are lower bounds" : "");
  }
}
