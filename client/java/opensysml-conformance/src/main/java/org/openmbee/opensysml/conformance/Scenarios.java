package org.openmbee.opensysml.conformance;

import com.google.gson.JsonArray;
import com.google.gson.JsonElement;
import com.google.gson.JsonObject;
import com.google.gson.JsonParser;
import java.io.IOException;
import java.io.UncheckedIOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.Comparator;
import java.util.HashMap;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.Set;
import java.util.stream.Stream;

/** Reads the scenario files, in file then declaration order. */
final class Scenarios {

  private static final String EXPECT = "expect";
  private static final String EXPECT_WITHOUT_CAPABILITY = "expect_without_capability";
  private static final String MODEL = "model";
  private static final String REQUEST = "request";
  private static final String RESPONSE = "response";
  private static final String STRICT_CONFORMANCE = "strict_conformance";

  private static final Set<String> SCENARIO_MEMBERS =
      Set.of(
          "id",
          "description",
          "rpc",
          "requires_capabilities",
          EXPECT_WITHOUT_CAPABILITY,
          MODEL,
          REQUEST,
          EXPECT);
  private static final Set<String> EXPECT_MEMBERS =
      Set.of(
          "status",
          "status_message_contains",
          RESPONSE,
          "contains",
          "contains_all",
          "non_empty",
          "absent",
          "counts",
          "min_counts");
  private static final Set<String> MODEL_MEMBERS =
      Set.of("fixture", "fixtures", "language", STRICT_CONFORMANCE);

  private Scenarios() {}

  /**
   * Every scenario in a directory of scenario files.
   *
   * @param directory the {@code scenarios} directory
   * @return the scenarios, in file then declaration order
   */
  static List<Scenario> load(Path directory) {
    List<Path> files;
    try (Stream<Path> entries = Files.list(directory)) {
      files =
          entries
              .filter(path -> path.getFileName().toString().endsWith(".json"))
              .sorted(Comparator.comparing(Path::toString))
              .toList();
    } catch (IOException e) {
      throw new UncheckedIOException("reading " + directory, e);
    }
    if (files.isEmpty()) {
      throw new IllegalArgumentException("no scenario files in " + directory);
    }

    List<Scenario> scenarios = new ArrayList<>();
    Map<String, String> seen = new HashMap<>();
    for (Path file : files) {
      JsonObject suite = read(file).getAsJsonObject();
      for (JsonElement element : suite.getAsJsonArray("scenarios")) {
        Scenario scenario = scenario(element.getAsJsonObject(), file);
        String duplicate = seen.put(scenario.id(), file.toString());
        if (duplicate != null) {
          throw new IllegalArgumentException(
              file + ": scenario id " + scenario.id() + " is already declared in " + duplicate);
        }
        scenarios.add(scenario);
      }
    }
    return List.copyOf(scenarios);
  }

  private static JsonElement read(Path file) {
    try {
      return JsonParser.parseString(Files.readString(file, StandardCharsets.UTF_8));
    } catch (IOException e) {
      throw new UncheckedIOException("reading " + file, e);
    }
  }

  private static Scenario scenario(JsonObject object, Path file) {
    reject(object, SCENARIO_MEMBERS, file + ": scenario");
    String id = text(object, "id");
    String rpc = text(object, "rpc");
    if (id.isEmpty() || rpc.isEmpty()) {
      throw new IllegalArgumentException(file + ": every scenario needs an id and an rpc");
    }
    return new Scenario(
        id,
        text(object, "description"),
        rpc,
        strings(object.get("requires_capabilities")),
        object.has(EXPECT_WITHOUT_CAPABILITY)
            ? Optional.of(expect(object.getAsJsonObject(EXPECT_WITHOUT_CAPABILITY), file))
            : Optional.empty(),
        object.has(MODEL)
            ? Optional.of(fixture(object.getAsJsonObject(MODEL), file))
            : Optional.empty(),
        object.has(REQUEST) ? object.getAsJsonObject(REQUEST) : new JsonObject(),
        object.has(EXPECT) ? expect(object.getAsJsonObject(EXPECT), file) : empty(),
        file.toString());
  }

  private static Scenario.Fixture fixture(JsonObject model, Path file) {
    reject(model, MODEL_MEMBERS, file + ": model");
    return new Scenario.Fixture(
        text(model, "fixture"),
        strings(model.get("fixtures")),
        text(model, "language"),
        model.has(STRICT_CONFORMANCE) && model.get(STRICT_CONFORMANCE).getAsBoolean());
  }

  private static Scenario.Expect expect(JsonObject object, Path file) {
    reject(object, EXPECT_MEMBERS, file + ": expect");
    return new Scenario.Expect(
        text(object, "status"),
        text(object, "status_message_contains"),
        object.has(RESPONSE) ? Optional.of(object.getAsJsonObject(RESPONSE)) : Optional.empty(),
        textMap(object.get("contains")),
        stringsMap(object.get("contains_all")),
        strings(object.get("non_empty")),
        strings(object.get("absent")),
        numberMap(object.get("counts")),
        numberMap(object.get("min_counts")));
  }

  private static Scenario.Expect empty() {
    return new Scenario.Expect(
        "", "", Optional.empty(), Map.of(), Map.of(), List.of(), List.of(), Map.of(), Map.of());
  }

  private static void reject(JsonObject object, Set<String> known, String where) {
    for (String member : object.keySet()) {
      if (!known.contains(member)) {
        throw new IllegalArgumentException(where + ": unknown member " + member);
      }
    }
  }

  private static String text(JsonObject object, String member) {
    return object.has(member) ? object.get(member).getAsString() : "";
  }

  private static List<String> strings(JsonElement element) {
    if (element == null) {
      return List.of();
    }
    List<String> values = new ArrayList<>();
    for (JsonElement item : element.getAsJsonArray()) {
      values.add(item.getAsString());
    }
    return List.copyOf(values);
  }

  private static Map<String, String> textMap(JsonElement element) {
    if (element == null) {
      return Map.of();
    }
    Map<String, String> values = new LinkedHashMap<>();
    for (Map.Entry<String, JsonElement> entry : element.getAsJsonObject().entrySet()) {
      values.put(entry.getKey(), entry.getValue().getAsString());
    }
    return Map.copyOf(values);
  }

  private static Map<String, List<String>> stringsMap(JsonElement element) {
    if (element == null) {
      return Map.of();
    }
    Map<String, List<String>> values = new LinkedHashMap<>();
    for (Map.Entry<String, JsonElement> entry : element.getAsJsonObject().entrySet()) {
      JsonArray items = entry.getValue().getAsJsonArray();
      values.put(entry.getKey(), strings(items));
    }
    return Map.copyOf(values);
  }

  private static Map<String, JsonElement> numberMap(JsonElement element) {
    if (element == null) {
      return Map.of();
    }
    return Map.copyOf(element.getAsJsonObject().asMap());
  }
}
