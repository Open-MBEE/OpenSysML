package org.openmbee.opensysml.conformance;

import com.google.gson.JsonArray;
import com.google.gson.JsonElement;
import com.google.gson.JsonNull;
import com.google.gson.JsonObject;
import com.google.gson.JsonPrimitive;
import java.math.BigInteger;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.TreeMap;
import java.util.TreeSet;

/**
 * Compares a normalized response against an expectation, by the rules in conformance/README.md:
 * only what the expectation names is compared, an unset field and a default are the same, Reals
 * agree within a relative tolerance and integers agree exactly.
 */
final class Checks {

  private static final String WANT = ", want ";
  private static final String ENTRIES = " entries";

  /**
   * The relative difference two Reals may have and still be the same value. It applies to Reals
   * alone: a tolerance on an integral field would let large counts and ids differ and pass.
   */
  static final double TOLERANCE = 1e-9;

  private Checks() {}

  /**
   * Every way an answer disagrees with an expectation.
   *
   * @param expect what the answer must be
   * @param actual the normalized answer
   * @return one message per mismatch, in a stable order; empty when the answer holds
   */
  static List<String> check(Scenario.Expect expect, Map<String, Object> actual) {
    List<String> failures = new ArrayList<>();
    expect.response().ifPresent(response -> failures.addAll(match("", response, actual)));

    for (String path : new TreeSet<>(expect.nonEmpty())) {
      Optional<Object> value = lookup(actual, path);
      if (value.isEmpty()) {
        failures.add(path + ": not set, want a value");
      } else if (isEmpty(value.get())) {
        failures.add(path + ": empty, want a value");
      }
    }
    for (String path : new TreeSet<>(expect.absent())) {
      Optional<Object> value = lookup(actual, path);
      if (value.isPresent() && !isEmpty(value.get())) {
        failures.add(path + ": set to " + render(value.get()) + WANT + "it unset");
      }
    }
    for (Map.Entry<String, String> entry : new TreeMap<>(expect.contains()).entrySet()) {
      failures.addAll(contains(actual, entry.getKey(), entry.getValue()));
    }
    for (Map.Entry<String, List<String>> entry : new TreeMap<>(expect.containsAll()).entrySet()) {
      failures.addAll(containsAll(actual, entry.getKey(), entry.getValue()));
    }
    for (Map.Entry<String, JsonElement> entry : new TreeMap<>(expect.counts()).entrySet()) {
      int want = entry.getValue().getAsInt();
      OptionalCount got = count(actual, entry.getKey());
      if (!got.countable()) {
        failures.add(entry.getKey() + ": not a list or map, want " + want + ENTRIES);
      } else if (got.count() != want) {
        failures.add(entry.getKey() + ": " + got.count() + ENTRIES + WANT + want);
      }
    }
    for (Map.Entry<String, JsonElement> entry : new TreeMap<>(expect.minCounts()).entrySet()) {
      int want = entry.getValue().getAsInt();
      OptionalCount got = count(actual, entry.getKey());
      if (!got.countable()) {
        failures.add(entry.getKey() + ": not a list or map, want at least " + want + ENTRIES);
      } else if (got.count() < want) {
        failures.add(entry.getKey() + ": " + got.count() + ENTRIES + ", want at least " + want);
      }
    }
    return List.copyOf(failures);
  }

  /** The number of entries at a path, and whether there is something countable there. */
  private record OptionalCount(int count, boolean countable) {}

  private static List<String> contains(Map<String, Object> actual, String path, String want) {
    Optional<Object> value = lookup(actual, path);
    if (value.isEmpty()) {
      return List.of(path + ": not set, want it to contain \"" + want + "\"");
    }
    if (!(value.get() instanceof String text)) {
      return List.of(
          path + ": " + render(value.get()) + " is not text, want it to contain \"" + want + "\"");
    }
    if (!text.contains(want)) {
      return List.of(path + ": \"" + text + "\" does not contain \"" + want + "\"");
    }
    return List.of();
  }

  /**
   * Checks every wanted string is at a path: a substring of the text there, or a member of the
   * values there. A path may use {@code *} to take one field of every entry, as in {@code
   * elements.*.id}.
   */
  private static List<String> containsAll(
      Map<String, Object> actual, String path, List<String> wants) {
    Optional<Object> at = lookup(actual, path);
    if (at.isPresent() && at.get() instanceof String text) {
      List<String> failures = new ArrayList<>();
      for (String want : wants) {
        if (!text.contains(want)) {
          failures.add(path + ": does not contain \"" + want + "\"");
        }
      }
      return failures;
    }
    Optional<List<Object>> found = values(actual, path);
    if (found.isEmpty()) {
      return List.of(
          path + ": neither text nor a list, want it to contain " + render(List.copyOf(wants)));
    }
    List<String> failures = new ArrayList<>();
    for (String want : wants) {
      if (!found.get().contains(want)) {
        failures.add(path + ": " + render(found.get()) + " does not contain \"" + want + "\"");
      }
    }
    return failures;
  }

  /** The values at a path, expanding a {@code *} segment and a list at the end of the path. */
  private static Optional<List<Object>> values(Object tree, String path) {
    Optional<List<Object>> found = walk(tree, List.of(path.split("\\.", -1)));
    if (found.isPresent() && found.get().size() == 1 && found.get().get(0) instanceof List<?> list) {
      return Optional.of(List.copyOf(list));
    }
    return found;
  }

  private static Optional<List<Object>> walk(Object current, List<String> segments) {
    if (segments.isEmpty()) {
      List<Object> single = new ArrayList<>();
      single.add(current);
      return Optional.of(single);
    }
    List<String> rest = segments.subList(1, segments.size());
    if (segments.get(0).equals("*")) {
      List<Object> entries;
      if (current instanceof List<?> list) {
        entries = List.copyOf(list);
      } else if (current instanceof Map<?, ?> map) {
        entries = new ArrayList<>(new TreeMap<>(asStringKeyed(map)).values());
      } else {
        return Optional.empty();
      }
      List<Object> found = new ArrayList<>();
      for (Object entry : entries) {
        Optional<List<Object>> reached = walk(entry, rest);
        if (reached.isEmpty()) {
          return Optional.empty();
        }
        found.addAll(reached.get());
      }
      return Optional.of(found);
    }
    Optional<Object> value = lookup(current, segments.get(0));
    return value.isEmpty() ? Optional.empty() : walk(value.get(), rest);
  }

  /**
   * Compares an expected tree against the answer's, reporting the paths that differ. Only what the
   * expectation names is compared, so a field added to the schema later does not fail a scenario.
   */
  private static List<String> match(String path, JsonElement want, Object got) {
    if (want instanceof JsonObject expected) {
      if (!(got instanceof Map<?, ?> raw)) {
        return List.of(at(path) + ": " + render(got) + WANT + "an object");
      }
      Map<String, Object> actual = asStringKeyed(raw);
      List<String> failures = new ArrayList<>();
      for (String key : new TreeSet<>(expected.keySet())) {
        String child = join(path, key);
        JsonElement wanted = expected.get(key);
        if (!actual.containsKey(key)) {
          // An unset field and a field left at its default are the same thing on the wire.
          if (!isDefault(wanted)) {
            failures.add(child + ": not set" + WANT + render(wanted));
          }
        } else {
          failures.addAll(match(child, wanted, actual.get(key)));
        }
      }
      return failures;
    }
    if (want instanceof JsonArray expected) {
      if (!(got instanceof List<?> actual)) {
        return List.of(at(path) + ": " + render(got) + WANT + "a list");
      }
      if (actual.size() != expected.size()) {
        return List.of(
            at(path)
                + ": "
                + actual.size()
                + ENTRIES
                + WANT
                + expected.size()
                + " ("
                + render(got)
                + ")");
      }
      List<String> failures = new ArrayList<>();
      for (int i = 0; i < expected.size(); i++) {
        failures.addAll(match(join(path, String.valueOf(i)), expected.get(i), actual.get(i)));
      }
      return failures;
    }
    if (want instanceof JsonPrimitive primitive && primitive.isNumber()) {
      return matchNumber(path, primitive.getAsString(), got);
    }
    String wanted = want instanceof JsonNull ? "null" : ((JsonPrimitive) want).getAsString();
    if (!wanted.equals(text(got))) {
      return List.of(at(path) + ": " + render(got) + WANT + render(want));
    }
    return List.of();
  }

  /** Compares an expected number, as the literal a scenario holds, against the answer's value. */
  private static List<String> matchNumber(String path, String want, Object got) {
    if (got instanceof Long integral) {
      return matchIntegral(path, want, integral.toString());
    }
    if (got instanceof BigInteger unsigned) {
      return matchIntegral(path, want, unsigned.toString());
    }
    if (got instanceof Double real) {
      Optional<Double> number = parseDouble(want);
      if (number.isEmpty() || !near(real, number.get())) {
        return List.of(at(path) + ": " + real + WANT + want);
      }
      return List.of();
    }
    return List.of(at(path) + ": " + render(got) + WANT + "the number " + want);
  }

  /** Compares an integral field by its digits, so nothing is lost to floating point on the way. */
  private static List<String> matchIntegral(String path, String want, String got) {
    Optional<String> digits = integerLiteral(want);
    if (digits.isEmpty() || !digits.get().equals(got)) {
      return List.of(at(path) + ": " + got + WANT + want);
    }
    return List.of();
  }

  /** An expected number as the digits of a whole number, taking "1500.0" and "1.5e3" too. */
  private static Optional<String> integerLiteral(String text) {
    try {
      return Optional.of(Long.toString(Long.parseLong(text)));
    } catch (NumberFormatException ignored) {
      // Not a signed literal; a uint64 above Long.MAX_VALUE still is one.
    }
    try {
      BigInteger unsigned = new BigInteger(text);
      if (unsigned.signum() >= 0 && unsigned.bitLength() <= 64) {
        return Optional.of(unsigned.toString());
      }
      return Optional.empty();
    } catch (NumberFormatException ignored) {
      // Not integral digits; "1500.0" is handled below.
    }
    Optional<Double> number = parseDouble(text);
    if (number.isEmpty()) {
      return Optional.empty();
    }
    double value = number.get();
    if (value != Math.rint(value) || Math.abs(value) >= 0x1p63) {
      return Optional.empty();
    }
    return Optional.of(Long.toString((long) value));
  }

  private static Optional<Double> parseDouble(String text) {
    try {
      return Optional.of(Double.valueOf(text));
    } catch (NumberFormatException e) {
      return Optional.empty();
    }
  }

  /** Whether two Reals are the same within {@link #TOLERANCE}. */
  private static boolean near(double got, double want) {
    if (got == want) {
      return true;
    }
    double scale = Math.max(Math.abs(got), Math.abs(want));
    return Math.abs(got - want) <= TOLERANCE * scale;
  }

  /** Whether an expected value is what an unset field holds. */
  private static boolean isDefault(JsonElement want) {
    if (want.isJsonNull()) {
      return true;
    }
    if (want instanceof JsonArray array) {
      return array.isEmpty();
    }
    if (want instanceof JsonObject object) {
      return object.isEmpty();
    }
    JsonPrimitive primitive = (JsonPrimitive) want;
    if (primitive.isBoolean()) {
      return !primitive.getAsBoolean();
    }
    if (primitive.isNumber()) {
      return parseDouble(primitive.getAsString()).orElse(Double.NaN) == 0;
    }
    return primitive.getAsString().isEmpty();
  }

  /** Whether a normalized value is what an unset field holds. */
  static boolean isEmpty(Object value) {
    if (value == null) {
      return true;
    }
    if (value instanceof Boolean flag) {
      return !flag;
    }
    if (value instanceof Number number) {
      return number.doubleValue() == 0;
    }
    if (value instanceof String text) {
      return text.isEmpty();
    }
    if (value instanceof List<?> list) {
      return list.isEmpty();
    }
    if (value instanceof Map<?, ?> map) {
      return map.isEmpty();
    }
    return false;
  }

  /**
   * The value a dotted path reaches: field names, map keys and list indices, as in {@code
   * instances.0.feature_values.mass.value.real_value}.
   *
   * @param tree the normalized answer, or a subtree of it
   * @param path the dotted path
   * @return the value there, or empty when the path reaches nothing
   */
  static Optional<Object> lookup(Object tree, String path) {
    Object current = tree;
    for (String segment : path.split("\\.", -1)) {
      if (current instanceof Map<?, ?> map) {
        Map<String, Object> keyed = asStringKeyed(map);
        if (!keyed.containsKey(segment)) {
          return Optional.empty();
        }
        current = keyed.get(segment);
      } else if (current instanceof List<?> list) {
        int index;
        try {
          index = Integer.parseInt(segment);
        } catch (NumberFormatException e) {
          return Optional.empty();
        }
        if (index < 0 || index >= list.size()) {
          return Optional.empty();
        }
        current = list.get(index);
      } else {
        return Optional.empty();
      }
    }
    return Optional.ofNullable(current);
  }

  private static OptionalCount count(Map<String, Object> tree, String path) {
    Optional<Object> value = lookup(tree, path);
    if (value.isEmpty()) {
      // An empty list or map is an unset field, so nothing there is zero entries.
      return new OptionalCount(0, true);
    }
    if (value.get() instanceof List<?> list) {
      return new OptionalCount(list.size(), true);
    }
    if (value.get() instanceof Map<?, ?> map) {
      return new OptionalCount(map.size(), true);
    }
    return new OptionalCount(0, false);
  }

  /** A normalized tree's maps are keyed by field or map-key name, so this copy never loses one. */
  private static Map<String, Object> asStringKeyed(Map<?, ?> map) {
    Map<String, Object> keyed = new LinkedHashMap<>();
    map.forEach((key, value) -> keyed.put(String.valueOf(key), value));
    return keyed;
  }

  private static String at(String path) {
    return path.isEmpty() ? "response" : path;
  }

  private static String join(String path, String segment) {
    return path.isEmpty() ? segment : path + "." + segment;
  }

  private static String text(Object value) {
    return value == null ? "null" : String.valueOf(value);
  }

  /** A value as a scenario would write it. */
  static String render(Object value) {
    if (value == null) {
      return "nothing";
    }
    if (value instanceof String text) {
      return "\"" + text + "\"";
    }
    if (value instanceof JsonPrimitive primitive) {
      return primitive.isString() ? "\"" + primitive.getAsString() + "\"" : primitive.getAsString();
    }
    if (value instanceof JsonNull) {
      return "nothing";
    }
    if (value instanceof JsonObject object) {
      List<String> parts = new ArrayList<>();
      for (String key : new TreeSet<>(object.keySet())) {
        parts.add(key + ": " + render(object.get(key)));
      }
      return "{" + String.join(", ", parts) + "}";
    }
    if (value instanceof JsonArray array) {
      List<String> parts = new ArrayList<>();
      array.forEach(item -> parts.add(render(item)));
      return "[" + String.join(", ", parts) + "]";
    }
    if (value instanceof Map<?, ?> map) {
      List<String> parts = new ArrayList<>();
      for (Map.Entry<String, Object> entry : new TreeMap<>(asStringKeyed(map)).entrySet()) {
        parts.add(entry.getKey() + ": " + render(entry.getValue()));
      }
      return "{" + String.join(", ", parts) + "}";
    }
    if (value instanceof List<?> list) {
      List<String> parts = new ArrayList<>();
      list.forEach(item -> parts.add(render(item)));
      return "[" + String.join(", ", parts) + "]";
    }
    return String.valueOf(value);
  }
}
