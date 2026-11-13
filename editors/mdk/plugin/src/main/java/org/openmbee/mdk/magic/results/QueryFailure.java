package org.openmbee.mdk.magic.results;

import java.util.ArrayList;
import java.util.List;
import java.util.function.Function;
import java.util.function.UnaryOperator;
import org.openmbee.opensysml.QueryOutcome;
import org.openmbee.opensysml.QueryOutcomeException;
import org.openmbee.opensysml.ServiceException;
import org.openmbee.mdk.magic.results.RunResult.Outcome;
import org.openmbee.mdk.magic.results.RunResult.Status;

/** Maps a failed query to outcomes: FAILED ones for the path or model error it reported, one ERROR for any other refusal. */
public final class QueryFailure {
  private QueryFailure() {}

  /**
   * The query's own outcome with the service's message, then one per finding and per path failure at the element it names.
   * Element ids are the service's names, as every outcome's are; the text names each element as {@code hostName} gives it.
   */
  public static List<Outcome> outcomes(String query, RuntimeException failure, UnaryOperator<String> hostName) {
    if (failure instanceof QueryOutcomeException reported
        && (reported.outcome().isPathError() || reported.outcome().isModelError())) {
      return reported(query, reported.serviceMessage(), reported.outcome(), hostName);
    }
    String message = failure instanceof ServiceException service ? service.serviceMessage() : failure.getMessage();
    return List.of(new Outcome(query, message == null ? failure.getClass().getName() : message, Status.ERROR, query));
  }

  private static List<Outcome> reported(String query, String message, QueryOutcome outcome, UnaryOperator<String> hostName) {
    List<Outcome> outcomes = new ArrayList<>();
    outcomes.add(new Outcome(query, message, Status.FAILED, query));
    for (QueryOutcome.Finding finding : outcome.findings()) {
      outcomes.add(new Outcome(query, describe(finding, hostName), Status.FAILED, element(finding.element())));
    }
    unlisted(outcomes, query, outcome.findingsTotal() - outcome.findings().size(), "model error", outcome.step());
    List<QueryOutcome.Failure> failures = new ArrayList<>(outcome.failures());
    failures.addAll(outcome.pathFailures());
    for (QueryOutcome.Failure failure : failures) {
      outcomes.add(new Outcome(query, describe(failure, hostName), Status.FAILED, element(failure.frontier().at())));
    }
    unlisted(outcomes, query, outcome.failuresTotal() + outcome.pathFailuresTotal() - failures.size(), "path error", outcome.step());
    return outcomes;
  }

  /** The failures and findings the service counted but did not send get one outcome saying how many. */
  private static void unlisted(List<Outcome> outcomes, String query, int count, String kind, String step) {
    if (count <= 0) return;
    outcomes.add(new Outcome(
        query, "… and " + count + " more " + kind + (count == 1 ? "" : "s") + " at " + step, Status.FAILED, null));
  }

  static String describe(QueryOutcome.Finding finding, UnaryOperator<String> hostName) {
    return "model error at " + finding.step() + ": " + hostName.apply(finding.element()) + " violates "
        + finding.constraint() + ": " + finding.observed();
  }

  /** A path failure on one line: the step and what it expected, then where it stopped and what is there. */
  static String describe(QueryOutcome.Failure failure, UnaryOperator<String> hostName) {
    QueryOutcome.Frontier frontier = failure.frontier();
    StringBuilder text = new StringBuilder("path error at ").append(failure.step())
        .append(": expected ").append(failure.expected());
    if (!frontier.at().isEmpty()) text.append("; at ").append(hostName.apply(frontier.at()));
    candidates(text, "members", frontier.members(), frontier.membersTotal(),
        member -> member.name() + " (" + member.metaclass() + ")");
    candidates(text, "features", frontier.features(), frontier.featuresTotal(),
        feature -> feature.type().isEmpty() ? feature.name() : feature.name() + " : " + feature.type());
    candidates(text, "kinds", frontier.kinds(), frontier.kindsTotal(), kind -> kind);
    if (!frontier.nearest().isEmpty()) text.append("; nearest: ").append(frontier.nearest());
    return text.toString();
  }

  /** A candidate list with those the service cut counted after it; a list of none is left out. */
  private static <T> void candidates(
      StringBuilder text, String name, List<T> listed, int total, Function<T, String> render) {
    if (total == 0) return;
    text.append("; ").append(name).append(": [")
        .append(String.join(", ", listed.stream().map(render).toList()));
    if (total > listed.size()) {
      text.append(listed.isEmpty() ? "" : ", ").append("… and ").append(total - listed.size()).append(" more");
    }
    text.append("]");
  }

  private static String element(String qualifiedName) {
    return qualifiedName.isEmpty() ? null : qualifiedName;
  }
}
