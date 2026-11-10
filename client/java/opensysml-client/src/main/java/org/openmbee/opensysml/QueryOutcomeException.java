package org.openmbee.opensysml;

import java.util.Objects;

/**
 * A document query the service refused with a structured {@link QueryOutcome}: the model has no
 * answer for a query step (a path error) or breaks a constraint the query relies on (a model
 * error). The status and the service's message are those of any {@link ServiceException}; the
 * outcome adds what the message says in a form a caller can act on.
 */
public class QueryOutcomeException extends ServiceException {

  private static final long serialVersionUID = 1L;

  private final QueryOutcome outcome;
  private final String resultView;

  /**
   * Creates the exception.
   *
   * @param status the status the service answered with
   * @param message the service's message
   * @param outcome the decoded outcome
   */
  public QueryOutcomeException(StatusCode status, String message, QueryOutcome outcome) {
    this(status, message, outcome, "");
  }

  /**
   * Creates the failure of a query that was asked for its view.
   *
   * @param status the status the service refused with
   * @param message the service's wording
   * @param outcome the outcome
   * @param resultView the outcome as a {@code QVTS::QueryResult} view, or empty
   */
  public QueryOutcomeException(
      StatusCode status, String message, QueryOutcome outcome, String resultView) {
    super(status, message);
    this.outcome = Objects.requireNonNull(outcome, "outcome");
    this.resultView = Objects.requireNonNull(resultView, "resultView");
  }

  /**
   * The failed query as a {@code QVTS::QueryResult} view: it exposes the frontier and finding
   * elements and states the outcome. Empty unless the call asked for the view.
   *
   * @return the view text, or empty
   */
  public String resultView() {
    return resultView;
  }

  /**
   * The structured outcome the service attached to the status.
   *
   * @return the outcome
   */
  public QueryOutcome outcome() {
    return outcome;
  }
}
