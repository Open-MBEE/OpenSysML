package org.openmbee.opensysml;

import java.util.ArrayList;
import java.util.List;

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
 */
public record Exploration(
    List<Outcome> outcomes,
    boolean complete,
    int runs,
    List<String> budgetsHit,
    int runsBudget,
    int depthBudget) {

  /**
   * Creates an exploration, copying its collections.
   *
   * @param outcomes the outcomes
   * @param complete whether the search completed
   * @param runs the runs made
   * @param budgetsHit the budgets that ended it
   * @param runsBudget the run budget
   * @param depthBudget the depth budget
   */
  public Exploration {
    outcomes = List.copyOf(outcomes);
    budgetsHit = List.copyOf(budgetsHit);
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
    return "incomplete: " + String.join(" and ", named) + " hit after " + runs + " runs";
  }
}
