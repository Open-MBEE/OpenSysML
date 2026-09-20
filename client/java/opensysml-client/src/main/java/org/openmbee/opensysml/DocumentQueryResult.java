package org.openmbee.opensysml;

import java.util.List;

/**
 * A document query's answer: its projected columns and typed rows, both in the deterministic order
 * the engine reports. A query that selects nothing answers with no rows; one that could not be run
 * fails the call with a status, which is a {@link ServiceException}.
 *
 * @param columns the projected property names, in projection order
 * @param rows the selected rows, in the engine's order
 */
public record DocumentQueryResult(List<String> columns, List<DocumentRow> rows) {

  /**
   * Creates a result, copying its collections.
   *
   * @param columns the column names
   * @param rows the rows
   */
  public DocumentQueryResult {
    columns = List.copyOf(columns);
    rows = List.copyOf(rows);
  }
}
