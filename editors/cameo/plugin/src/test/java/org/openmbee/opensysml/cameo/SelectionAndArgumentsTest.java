package org.openmbee.opensysml.cameo;

import static org.junit.jupiter.api.Assertions.assertEquals;
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
  void normalizesQuotedAndEscapedSegments() {
    assertEquals("A::B C::D'E", QualifiedNames.normalize("A::'B C'::'D\\'E'"));
    assertEquals("A::B", QualifiedNames.normalize(" A :: B "));
  }
}
