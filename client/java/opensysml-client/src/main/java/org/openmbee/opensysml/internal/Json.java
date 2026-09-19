package org.openmbee.opensysml.internal;

import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

/**
 * A JSON reader for the one JSON the client must read: the Connect protocol's error body.
 *
 * <p>Small enough to keep {@code protobuf-java} the client's only required dependency, which
 * matters for a jar loaded into a host application's classloader. It is a complete reader of the
 * grammar, not a scanner for two fields.
 */
public final class Json {

  private final String text;
  private int position;

  private Json(String text) {
    this.text = text;
  }

  /**
   * Parses a JSON document.
   *
   * @param text the document
   * @return a {@link Map}, {@link List}, {@link String}, {@link Double}, {@link Boolean} or
   *     {@code null}
   * @throws IllegalArgumentException if the document is not well-formed JSON
   */
  public static Object parse(String text) {
    Json reader = new Json(text);
    reader.skipWhitespace();
    Object value = reader.readValue();
    reader.skipWhitespace();
    if (reader.position != text.length()) {
      throw reader.error("trailing content");
    }
    return value;
  }

  /**
   * The string value of a member of a JSON object.
   *
   * @param document a parsed document
   * @param member the member name
   * @return the string, or {@code ""} when the document is not an object, the member is missing or
   *     its value is not a string
   */
  public static String stringMember(Object document, String member) {
    if (document instanceof Map<?, ?> object) {
      Object value = object.get(member);
      if (value instanceof String string) {
        return string;
      }
    }
    return "";
  }

  private Object readValue() {
    if (position >= text.length()) {
      throw error("unexpected end of document");
    }
    char c = text.charAt(position);
    return switch (c) {
      case '{' -> readObject();
      case '[' -> readArray();
      case '"' -> readString();
      case 't' -> readLiteral("true", Boolean.TRUE);
      case 'f' -> readLiteral("false", Boolean.FALSE);
      case 'n' -> readLiteral("null", null);
      default -> readNumber();
    };
  }

  private Map<String, Object> readObject() {
    Map<String, Object> members = new LinkedHashMap<>();
    expect('{');
    skipWhitespace();
    if (peek() == '}') {
      position++;
      return members;
    }
    while (true) {
      skipWhitespace();
      String name = readString();
      skipWhitespace();
      expect(':');
      skipWhitespace();
      members.put(name, readValue());
      skipWhitespace();
      char c = peek();
      position++;
      if (c == '}') {
        return members;
      }
      if (c != ',') {
        throw error("expected ',' or '}'");
      }
    }
  }

  private List<Object> readArray() {
    List<Object> elements = new ArrayList<>();
    expect('[');
    skipWhitespace();
    if (peek() == ']') {
      position++;
      return elements;
    }
    while (true) {
      skipWhitespace();
      elements.add(readValue());
      skipWhitespace();
      char c = peek();
      position++;
      if (c == ']') {
        return elements;
      }
      if (c != ',') {
        throw error("expected ',' or ']'");
      }
    }
  }

  private String readString() {
    expect('"');
    StringBuilder value = new StringBuilder();
    while (true) {
      if (position >= text.length()) {
        throw error("unterminated string");
      }
      char c = text.charAt(position++);
      if (c == '"') {
        return value.toString();
      }
      if (c != '\\') {
        value.append(c);
        continue;
      }
      if (position >= text.length()) {
        throw error("unterminated escape");
      }
      char escaped = text.charAt(position++);
      switch (escaped) {
        case '"', '\\', '/' -> value.append(escaped);
        case 'b' -> value.append('\b');
        case 'f' -> value.append('\f');
        case 'n' -> value.append('\n');
        case 'r' -> value.append('\r');
        case 't' -> value.append('\t');
        case 'u' -> {
          if (position + 4 > text.length()) {
            throw error("truncated unicode escape");
          }
          value.append((char) Integer.parseInt(text.substring(position, position + 4), 16));
          position += 4;
        }
        default -> throw error("unknown escape \\" + escaped);
      }
    }
  }

  private Object readLiteral(String literal, Object value) {
    if (!text.startsWith(literal, position)) {
      throw error("expected " + literal);
    }
    position += literal.length();
    return value;
  }

  private Double readNumber() {
    int start = position;
    while (position < text.length() && "+-.eE0123456789".indexOf(text.charAt(position)) >= 0) {
      position++;
    }
    try {
      return Double.valueOf(text.substring(start, position));
    } catch (NumberFormatException e) {
      throw error("not a number: " + text.substring(start, position));
    }
  }

  private void expect(char c) {
    if (peek() != c) {
      throw error("expected '" + c + "'");
    }
    position++;
  }

  private char peek() {
    if (position >= text.length()) {
      throw error("unexpected end of document");
    }
    return text.charAt(position);
  }

  private void skipWhitespace() {
    while (position < text.length() && Character.isWhitespace(text.charAt(position))) {
      position++;
    }
  }

  private IllegalArgumentException error(String message) {
    return new IllegalArgumentException("invalid JSON at offset " + position + ": " + message);
  }
}
