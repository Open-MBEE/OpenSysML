package org.openmbee.opensysml;

import java.util.List;
import java.util.Objects;

/**
 * A query's selection with the {@code QVTS::QueryResult} view of it, as {@link
 * Model#queryAsView(Query)} and {@link Model#queryOslcAsView(String)} answer.
 *
 * @param elements the elements selected, as {@link Model#query(Query)} answers them
 * @param resultView the selection as SysML text: one view exposing each element and stating the
 *     query, its source, the outcome and the provenance
 */
public record QuerySelection(List<QueryElement> elements, String resultView) {

  /**
   * Creates a selection, copying its elements.
   *
   * @param elements the elements selected
   * @param resultView the view text
   */
  public QuerySelection {
    elements = List.copyOf(elements);
    Objects.requireNonNull(resultView, "resultView");
  }
}
