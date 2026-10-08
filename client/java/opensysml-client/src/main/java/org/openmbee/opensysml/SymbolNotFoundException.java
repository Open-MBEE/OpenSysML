package org.openmbee.opensysml;

import java.util.List;
import java.util.Objects;

/** A lookup that required a symbol found none, as {@link Model#lookup(String)} reports. */
public class SymbolNotFoundException extends ModelException {

  private static final long serialVersionUID = 1L;

  private final String name;
  private final List<String> suggestions;

  /**
   * Creates the exception.
   *
   * @param name the name looked up
   * @param suggestions declared names close enough to be typos of it
   */
  public SymbolNotFoundException(String name, List<String> suggestions) {
    super(message(name, suggestions), List.of());
    this.name = name;
    this.suggestions = List.copyOf(suggestions);
  }

  private static String message(String name, List<String> suggestions) {
    Objects.requireNonNull(name, "name");
    String message = "no symbol named '" + name + "' in this model";
    if (!suggestions.isEmpty()) {
      message +=
          "; did you mean "
              + String.join(", ", suggestions.stream().map(s -> "'" + s + "'").toList())
              + "?";
    }
    return message;
  }

  /**
   * The name looked up.
   *
   * @return the name
   */
  public String name() {
    return name;
  }

  /**
   * Declared names close enough to be typos of {@link #name()}, best first.
   *
   * @return at most three names
   */
  public List<String> suggestions() {
    return suggestions;
  }
}
