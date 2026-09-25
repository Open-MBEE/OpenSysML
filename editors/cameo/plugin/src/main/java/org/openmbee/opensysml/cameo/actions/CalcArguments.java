package org.openmbee.opensysml.cameo.actions;

import java.util.ArrayList;
import java.util.List;
import org.openmbee.opensysml.Value;

/** Parses the comma-separated argument list typed for "Evaluate calc". */
public final class CalcArguments {
  private CalcArguments() {}

  public static List<Value> parse(String text) {
    List<Value> values = new ArrayList<>();
    if (text == null || text.isBlank()) return values;
    StringBuilder token = new StringBuilder();
    boolean inQuote = false;
    int i = 0;
    while (i < text.length()) {
      char current = text.charAt(i);
      if (inQuote && current == '\\' && i + 1 < text.length()) {
        token.append(current).append(text.charAt(i + 1));
        i++;
      } else if (current == '"') {
        inQuote = !inQuote;
        token.append(current);
      } else if (current == ',' && !inQuote) {
        values.add(value(token.toString().trim()));
        token.setLength(0);
      } else {
        token.append(current);
      }
      i++;
    }
    if (inQuote) throw new IllegalArgumentException("unterminated string in calc arguments");
    if (!token.isEmpty()) values.add(value(token.toString().trim()));
    return values;
  }

  static Value value(String text) {
    if (text.equalsIgnoreCase("true") || text.equalsIgnoreCase("false")) {
      return new Value.BooleanValue(Boolean.parseBoolean(text));
    }
    if (text.length() >= 2 && text.startsWith("\"") && text.endsWith("\"")) {
      StringBuilder unescaped = new StringBuilder();
      String contents = text.substring(1, text.length() - 1);
      int i = 0;
      while (i < contents.length()) {
        char current = contents.charAt(i);
        if (current == '\\' && i + 1 < contents.length()
            && (contents.charAt(i + 1) == '"' || contents.charAt(i + 1) == '\\')) {
          unescaped.append(contents.charAt(i + 1));
          i++;
        } else {
          unescaped.append(current);
        }
        i++;
      }
      return new Value.StringValue(unescaped.toString());
    }
    try {
      return new Value.IntegerValue(Long.parseLong(text));
    } catch (NumberFormatException notInteger) {
      try {
        return new Value.RealValue(Double.parseDouble(text));
      } catch (NumberFormatException notReal) {
        return new Value.StringValue(text);
      }
    }
  }
}
