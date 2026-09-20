package org.openmbee.opensysml;

import java.util.List;
import java.util.Objects;
import java.util.Optional;

/**
 * One selected element of a document query and its projected cells, one per column.
 *
 * @param element the selected element itself; for an object row, the usage the object is held
 *     under; for a row a {@code Verdicts} query answered, the assertion checked; for a state or
 *     event row, the usage of the object the row is about
 * @param cells one value sequence per column, in column order
 * @param verdict the verdict a row a {@code Verdicts} query answered carries; absent for any other
 *     row
 * @param object the object a row over held objects is about — one an {@code Objects} query
 *     enumerated, a bound object's part, or the object a state or event row is about; absent for
 *     any other row
 * @param state the state a row a {@code States} query answered carries; absent for any other row
 * @param event the event a row an {@code Events} query answered carries; absent for any other row
 */
public record DocumentRow(
    DocumentValue.ElementRef element,
    List<List<DocumentValue>> cells,
    Optional<DocumentValue.DocumentVerdict> verdict,
    Optional<DocumentValue.ObjectRef> object,
    Optional<DocumentValue.DocumentState> state,
    Optional<DocumentValue.DocumentEvent> event) {

  /**
   * Creates a row, copying its cells.
   *
   * @param element the selected element, never {@code null}
   * @param cells the cells, in column order
   * @param verdict the verdict, when the row is one
   * @param object the object, when the row is about one
   * @param state the state, when the row is one
   * @param event the event, when the row is one
   */
  public DocumentRow {
    Objects.requireNonNull(element, "element");
    cells = cells.stream().map(List::copyOf).toList();
    Objects.requireNonNull(verdict, "verdict");
    Objects.requireNonNull(object, "object");
    Objects.requireNonNull(state, "state");
    Objects.requireNonNull(event, "event");
  }
}
