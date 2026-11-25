package org.openmbee.opensysml;

import java.util.Objects;

/**
 * An element the model's element form knows: one of the model, or a standard-library element it
 * references.
 *
 * @param id the element form's {@code @id}, which {@link Model#elementProperties} accepts back as
 *     the element to read
 * @param qualifiedName the qualified name; empty for an unnamed element, and the library path
 *     ({@code owner/Metaclass[i]}) of an unnamed standard-library member
 * @param metaclass {@code "PartDefinition"}, {@code "ConnectionUsage"}, …; empty when the id is
 *     known to nothing
 */
public record ElementRef(String id, String qualifiedName, String metaclass) {
  /**
   * Creates an element reference.
   *
   * @param id the element form id, never {@code null}
   * @param qualifiedName the qualified name, empty when unnamed
   * @param metaclass the metaclass, empty when unknown
   */
  public ElementRef {
    Objects.requireNonNull(id, "id");
    Objects.requireNonNull(qualifiedName, "qualifiedName");
    Objects.requireNonNull(metaclass, "metaclass");
  }

  /**
   * The qualified name, or the id for an unnamed element.
   *
   * @return how the element is best named
   */
  @Override
  public String toString() {
    return qualifiedName.isEmpty() ? id : qualifiedName;
  }
}
