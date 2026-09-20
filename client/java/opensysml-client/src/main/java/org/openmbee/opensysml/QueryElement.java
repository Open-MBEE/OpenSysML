package org.openmbee.opensysml;

import java.util.Map;
import java.util.Objects;

/**
 * One element a {@link Query} selected.
 *
 * @param id the element's qualified name
 * @param type its metamodel type name ({@code "PartUsage"}, {@code "PartDefinition"}, ...)
 * @param properties what the query selected, omitting a property the element does not have
 */
public record QueryElement(String id, String type, Map<String, String> properties) {

  /**
   * Creates a query element, copying its properties.
   *
   * @param id the qualified name, never {@code null}
   * @param type the type name, never {@code null}
   * @param properties the properties
   */
  public QueryElement {
    Objects.requireNonNull(id, "id");
    Objects.requireNonNull(type, "type");
    properties = Map.copyOf(properties);
  }
}
