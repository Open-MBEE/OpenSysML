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
    for (String part : text.split(",")) values.add(value(part.trim()));
    return values;
  }

  static Value value(String text) {
    if (text.equalsIgnoreCase("true") || text.equalsIgnoreCase("false")) {
      return new Value.BooleanValue(Boolean.parseBoolean(text));
    }
    if (text.length() >= 2 && text.startsWith("\"") && text.endsWith("\"")) {
      return new Value.StringValue(text.substring(1, text.length() - 1));
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
