package org.openmbee.opensysml.conformance;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;

import java.math.BigInteger;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import org.junit.jupiter.api.Test;

/** The comparison rules of conformance/README.md, as this client applies them. */
class ChecksTest {

  private static Map<String, Object> tree(Object... pairs) {
    Map<String, Object> tree = new LinkedHashMap<>();
    for (int index = 0; index < pairs.length; index += 2) {
      tree.put((String) pairs[index], pairs[index + 1]);
    }
    return tree;
  }

  @Test
  void onlyWhatAnExpectationNamesIsCompared() {
    Map<String, Object> actual = tree("model_hash", "abc", "root", tree("kind", "partDef"));
    assertTrue(Checks.check(Expectations.response("{\"model_hash\": \"abc\"}"), actual).isEmpty());
  }

  @Test
  void aRealAgreesWithinTheRelativeTolerance() {
    double want = 1234.5;
    Map<String, Object> near = tree("value", want * (1 + Checks.TOLERANCE / 2));
    Map<String, Object> far = tree("value", want * (1 + Checks.TOLERANCE * 100));
    assertTrue(Checks.check(Expectations.response("{\"value\": 1234.5}"), near).isEmpty());
    assertEquals(1, Checks.check(Expectations.response("{\"value\": 1234.5}"), far).size());
  }

  @Test
  void anIntegerAgreesExactly() {
    assertTrue(Checks.check(Expectations.response("{\"n\": 4000000000001}"), tree("n", 4000000000001L)).isEmpty());
    assertEquals(
        1, Checks.check(Expectations.response("{\"n\": 4000000000001}"), tree("n", 4000000000002L)).size());
  }

  @Test
  void anIntegralValueMayBeWrittenAsAReal() {
    assertTrue(Checks.check(Expectations.response("{\"n\": 1500.0}"), tree("n", 1500L)).isEmpty());
    assertTrue(Checks.check(Expectations.response("{\"n\": 1.5e3}"), tree("n", 1500L)).isEmpty());
    assertTrue(
        Checks.check(Expectations.response("{\"n\": 3}"), tree("n", BigInteger.valueOf(3))).isEmpty());
  }

  @Test
  void anUnsetFieldAndItsDefaultAreTheSame() {
    assertTrue(Checks.check(Expectations.response("{\"error\": \"\"}"), tree()).isEmpty());
    assertTrue(Checks.check(Expectations.response("{\"count\": 0}"), tree()).isEmpty());
    assertTrue(Checks.check(Expectations.response("{\"flag\": false}"), tree()).isEmpty());
    assertTrue(Checks.check(Expectations.response("{\"items\": []}"), tree()).isEmpty());
    assertEquals(1, Checks.check(Expectations.response("{\"error\": \"boom\"}"), tree()).size());
  }

  @Test
  void aMapComparesRegardlessOfOrder() {
    Map<String, Object> actual = tree("features", tree("b", "2", "a", "1"));
    assertTrue(
        Checks.check(
                Expectations.response("{\"features\": {\"a\": \"1\", \"b\": \"2\"}}"), actual)
            .isEmpty());
  }

  @Test
  void aListMustHaveTheLengthTheExpectationNames() {
    Map<String, Object> actual = tree("items", List.of(tree("id", "@1"), tree("id", "@2")));
    assertTrue(
        Checks.check(
                Expectations.response("{\"items\": [{\"id\": \"@1\"}, {\"id\": \"@2\"}]}"), actual)
            .isEmpty());
    assertFalse(
        Checks.check(Expectations.response("{\"items\": [{\"id\": \"@1\"}]}"), actual).isEmpty());
  }

  @Test
  void containsAndContainsAllReadTextAndLists() {
    Map<String, Object> actual =
        tree(
            "error",
            "symbol not found: Missing",
            "members",
            List.of(tree("id", "A"), tree("id", "B")));
    assertTrue(
        Checks.check(
                Expectations.directives(
                    Map.of("error", "not found"),
                    Map.of("members.*.id", List.of("A", "B")),
                    List.of("error"),
                    List.of("root"),
                    Map.of("members", Expectations.number(2)),
                    Map.of("members", Expectations.number(1))),
                actual)
            .isEmpty());
    assertEquals(
        List.of("members.*.id: [\"A\", \"B\"] does not contain \"C\""),
        Checks.check(
            Expectations.directives(
                Map.of(),
                Map.of("members.*.id", List.of("C")),
                List.of(),
                List.of(),
                Map.of(),
                Map.of()),
            actual));
  }

  @Test
  void anAbsentPathMustBeUnsetAndANonEmptyOneMustHoldAValue() {
    Map<String, Object> actual = tree("error", "", "items", List.of());
    assertTrue(
        Checks.check(
                Expectations.directives(
                    Map.of(), Map.of(), List.of(), List.of("error", "items"), Map.of(), Map.of()),
                actual)
            .isEmpty());
    assertEquals(
        List.of("items: empty, want a value"),
        Checks.check(
            Expectations.directives(
                Map.of(), Map.of(), List.of("items"), List.of(), Map.of(), Map.of()),
            actual));
  }

  @Test
  void countsNeedSomethingCountable() {
    assertEquals(
        List.of("error: not a list or map, want 2 entries"),
        Checks.check(
            Expectations.directives(
                Map.of(),
                Map.of(),
                List.of(),
                List.of(),
                Map.of("error", Expectations.number(2)),
                Map.of()),
            tree("error", "boom")));
  }

  @Test
  void aFailureNamesThePathItDisagreesAbout() {
    assertEquals(
        List.of("root.kind: \"partUsage\", want \"partDef\""),
        Checks.check(
            Expectations.response("{\"root\": {\"kind\": \"partDef\"}}"),
            tree("root", tree("kind", "partUsage"))));
  }
}
