package org.openmbee.opensysml.conformance;

import com.google.gson.JsonElement;
import com.google.gson.JsonObject;
import com.google.gson.JsonParser;
import java.util.List;
import java.util.Map;
import java.util.Optional;

/** Builds the expectations the comparison tests state, from the JSON a scenario would carry. */
final class Expectations {

  private Expectations() {}

  /** An expectation naming a response tree, as a scenario's {@code expect.response} does. */
  static Scenario.Expect response(String json) {
    return new Scenario.Expect(
        "",
        "",
        Optional.of(object(json)),
        Map.of(),
        Map.of(),
        List.of(),
        List.of(),
        Map.of(),
        Map.of());
  }

  /** An expectation naming directives other than a response tree. */
  static Scenario.Expect directives(
      Map<String, String> contains,
      Map<String, List<String>> containsAll,
      List<String> nonEmpty,
      List<String> absent,
      Map<String, JsonElement> counts,
      Map<String, JsonElement> minCounts) {
    return new Scenario.Expect(
        "", "", Optional.empty(), contains, containsAll, nonEmpty, absent, counts, minCounts);
  }

  /** A JSON object. */
  static JsonObject object(String json) {
    return JsonParser.parseString(json).getAsJsonObject();
  }

  /** A JSON number, as a {@code counts} entry carries one. */
  static JsonElement number(int value) {
    return JsonParser.parseString(String.valueOf(value));
  }
}
