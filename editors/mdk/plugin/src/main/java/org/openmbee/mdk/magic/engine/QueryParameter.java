package org.openmbee.mdk.magic.engine;

import java.util.Objects;

/** One {@code in} parameter of a query definition; {@code hasValue} says the declaration binds a value, so it may run unbound. */
public record QueryParameter(
    String query, String name, String type, Kind kind, boolean many, boolean hasValue) {
  /** How a typed-in binding is read: anything but a scalar names an element. */
  public enum Kind {
    ELEMENT,
    STRING,
    INTEGER,
    REAL,
    BOOLEAN
  }

  public QueryParameter {
    Objects.requireNonNull(query);
    Objects.requireNonNull(name);
    Objects.requireNonNull(type);
    Objects.requireNonNull(kind);
  }

  /** The parameter must be bound for the query to run. */
  public boolean required() {
    return !hasValue;
  }

  /** The kind a declared type's primitive stands for; a type without one is an element. */
  public static Kind kindOf(String primitive) {
    if (primitive == null || primitive.isBlank()) return Kind.ELEMENT;
    String name = primitive.substring(primitive.lastIndexOf(':') + 1);
    return switch (name) {
      case "String" -> Kind.STRING;
      case "Integer", "Natural", "Positive" -> Kind.INTEGER;
      case "Real", "Rational", "Number", "Complex" -> Kind.REAL;
      case "Boolean" -> Kind.BOOLEAN;
      default -> Kind.ELEMENT;
    };
  }
}
