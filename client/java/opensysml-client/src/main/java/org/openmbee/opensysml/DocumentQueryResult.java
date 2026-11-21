package org.openmbee.opensysml;

import java.util.List;
import java.util.Objects;

/**
 * A document query's answer: its projected columns and typed rows, both in the deterministic order
 * the engine reports. A query that selects nothing answers with no rows; one that could not be run
 * fails the call with a status, which is a {@link ServiceException}.
 *
 * @param columns the projected property names, in projection order
 * @param rows the selected rows, in the engine's order
 * @param resultView the answer as a {@code QVTS::QueryResult} view, SysML text exposing each
 *     selected element; empty unless the call asked for it
 */
public record DocumentQueryResult(List<String> columns, List<DocumentRow> rows, String resultView) {

  /**
   * Creates a result, copying its collections.
   *
   * @param columns the column names
   * @param rows the rows
   * @param resultView the answer as a view, or empty
   */
  public DocumentQueryResult {
    columns = List.copyOf(columns);
    rows = List.copyOf(rows);
    Objects.requireNonNull(resultView, "resultView");
  }

  /**
   * Creates a result that carries no view.
   *
   * @param columns the column names
   * @param rows the rows
   */
  public DocumentQueryResult(List<String> columns, List<DocumentRow> rows) {
    this(columns, rows, "");
  }
}
