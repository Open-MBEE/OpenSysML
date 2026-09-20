package org.openmbee.opensysml.cameo.identity;

import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import org.openmbee.opensysml.cameo.model.ModelElement;

/** Qualified-name and id index over Cameo elements; ambiguous names return every candidate. */
public final class IdentityIndex implements IdentityResolver {
  private final Map<String, List<ModelElement>> byName;
  private final Map<String, List<ModelElement>> byId;

  private IdentityIndex(Map<String, List<ModelElement>> byName, Map<String, List<ModelElement>> byId) {
    this.byName = byName;
    this.byId = byId;
  }

  public static IdentityIndex of(List<? extends ModelElement> elements) {
    Map<String, List<ModelElement>> index = new LinkedHashMap<>();
    Map<String, List<ModelElement>> ids = new LinkedHashMap<>();
    for (ModelElement element : elements) {
      index.computeIfAbsent(QualifiedNames.normalize(element.qualifiedName()), ignored -> new ArrayList<>())
          .add(element);
      ids.computeIfAbsent(element.id(), ignored -> new ArrayList<>()).add(element);
    }
    return new IdentityIndex(index, ids);
  }

  public int size() {
    return byId.values().stream().mapToInt(List::size).sum();
  }

  @Override
  public List<ModelElement> resolve(String symbolId) {
    String normalized = QualifiedNames.normalize(symbolId);
    List<ModelElement> idMatches = byId.get(symbolId);
    if (idMatches != null) return List.copyOf(idMatches);
    List<ModelElement> exact = byName.get(normalized);
    if (exact != null) {
      return List.copyOf(exact);
    }
    int separator = normalized.lastIndexOf("::");
    if (separator >= 0) {
      String last = normalized.substring(separator + 2);
      int dot = last.indexOf('.');
      if (dot >= 0) {
        String prefix = normalized.substring(0, separator + 2) + last.substring(0, dot);
        List<ModelElement> candidates = byName.get(prefix);
        if (candidates != null) {
          return List.copyOf(candidates);
        }
      }
    }
    return List.of();
  }
}
