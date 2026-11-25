package org.openmbee.mdk.magic.results;

import java.util.Objects;
import java.util.Optional;
import org.openmbee.opensysml.DocumentQueryResult;
import org.openmbee.opensysml.QueryOutcomeException;

/** What running one query gave: its rows, or the failure the service answered. */
public record QueryAnswer(String query, Optional<DocumentQueryResult> result, Optional<RuntimeException> failure) {
  public QueryAnswer {
    Objects.requireNonNull(query);
    Objects.requireNonNull(result);
    Objects.requireNonNull(failure);
    if (result.isPresent() == failure.isPresent()) {
      throw new IllegalArgumentException("a query answer is rows or a failure");
    }
  }

  public static QueryAnswer rows(String query, DocumentQueryResult result) {
    return new QueryAnswer(query, Optional.of(result), Optional.empty());
  }

  public static QueryAnswer failed(String query, RuntimeException failure) {
    return new QueryAnswer(query, Optional.empty(), Optional.of(failure));
  }

  /** The answer as a {@code QVTS::QueryResult} view: of the rows, or of the outcome a failed query reported; empty when the service wrote none. */
  public Optional<RunResult.View> view() {
    String text = result.map(DocumentQueryResult::resultView)
        .orElseGet(() -> failure.get() instanceof QueryOutcomeException reported ? reported.resultView() : "");
    return ResultViews.named(text).map(name -> new RunResult.View(query, name, text));
  }
}
