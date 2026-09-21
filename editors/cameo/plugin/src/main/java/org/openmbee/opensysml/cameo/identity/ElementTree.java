package org.openmbee.opensysml.cameo.identity;

import java.util.ArrayDeque;
import java.util.ArrayList;
import java.util.Deque;
import java.util.List;
import java.util.Optional;
import java.util.function.Function;
import org.openmbee.opensysml.cameo.model.ModelElement;

/** Walks an ownership tree once and indexes every adaptable element by qualified name. */
public final class ElementTree {
  private ElementTree() {}

  public static <T> IdentityIndex index(
      T root, Function<T, ? extends Iterable<? extends T>> children, Function<T, Optional<ModelElement>> adapt) {
    List<ModelElement> elements = new ArrayList<>();
    Deque<T> pending = new ArrayDeque<>();
    pending.push(root);
    while (!pending.isEmpty()) {
      T current = pending.pop();
      adapt.apply(current).ifPresent(elements::add);
      for (T child : children.apply(current)) pending.push(child);
    }
    return IdentityIndex.of(elements);
  }
}
