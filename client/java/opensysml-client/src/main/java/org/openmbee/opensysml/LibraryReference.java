package org.openmbee.opensysml;

import java.util.Objects;

/**
 * A standard-library element an api-json conversion references outside its element array, in
 * place of the library stub releases before 0.11.0 wrote.
 *
 * @param id the element's id, as the conversion's references spell it
 * @param qualifiedName the element's qualified name, such as {@code SI::second}
 * @param type the element's metaclass, as API JSON's {@code @type} spells it
 */
public record LibraryReference(String id, String qualifiedName, String type) {
  public LibraryReference {
    Objects.requireNonNull(id, "id");
    Objects.requireNonNull(qualifiedName, "qualifiedName");
    Objects.requireNonNull(type, "type");
  }
}
