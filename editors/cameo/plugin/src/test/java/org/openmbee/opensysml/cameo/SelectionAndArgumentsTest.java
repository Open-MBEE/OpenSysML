package org.openmbee.opensysml.cameo;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

import java.util.List;
import org.junit.jupiter.api.Test;
import org.openmbee.opensysml.Value;
import org.openmbee.opensysml.cameo.actions.CalcArguments;
import org.openmbee.opensysml.cameo.identity.QualifiedNames;

class SelectionAndArgumentsTest {
  @Test
  void parsesCalcArgumentsByShape() {
    assertEquals(List.of(new Value.IntegerValue(3), new Value.RealValue(2.5), new Value.BooleanValue(true),
        new Value.StringValue("x y"), new Value.StringValue("name")), CalcArguments.parse("3, 2.5, true, \"x y\", name"));
    assertTrue(CalcArguments.parse("  ").isEmpty());
  }

  @Test
  void preservesCommasAndUnescapesQuotedStrings() {
    assertEquals(List.of(new Value.StringValue("Doe, Jane"), new Value.IntegerValue(3)),
        CalcArguments.parse("\"Doe, Jane\", 3"));
    assertEquals(List.of(new Value.StringValue("a\"b")), CalcArguments.parse("\"a\\\"b\""));
  }

  @Test
  void rejectsUnterminatedQuotedStrings() {
    IllegalArgumentException failure =
        assertThrows(IllegalArgumentException.class, () -> CalcArguments.parse("\"open"));
    assertEquals("unterminated string in calc arguments", failure.getMessage());
  }

  @Test
  void normalizesQuotedAndEscapedSegments() {
    assertEquals("A::B C::D'E", QualifiedNames.normalize("A::'B C'::'D\\'E'"));
    assertEquals("A::B", QualifiedNames.normalize(" A :: B "));
  }
}
