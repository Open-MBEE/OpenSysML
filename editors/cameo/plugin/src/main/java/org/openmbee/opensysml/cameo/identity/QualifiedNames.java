package org.openmbee.opensysml.cameo.identity;

import java.util.ArrayList;
import java.util.List;

public final class QualifiedNames {
  private QualifiedNames() {}

  public static String normalize(String value) {
    List<String> parts = new ArrayList<>();
    StringBuilder current = new StringBuilder();
    boolean quoted = false;
    for (int i = 0; i < value.length(); i++) {
      char c = value.charAt(i);
      if (c == '\\' && i + 1 < value.length() && value.charAt(i + 1) == '\'') {
        current.append('\'');
        i++;
      } else if (c == '\'') {
        quoted = !quoted;
      } else if (c == ':' && !quoted && i + 1 < value.length() && value.charAt(i + 1) == ':') {
        parts.add(current.toString().trim());
        current.setLength(0);
        i++;
      } else {
        current.append(c);
      }
    }
    parts.add(current.toString().trim());
    return join(parts);
  }

  public static String join(List<String> parts) {
    return parts.stream().map(String::trim).filter(s -> !s.isEmpty()).reduce((a, b) -> a + "::" + b)
        .orElse("");
  }
}
