package org.openmbee.mdk.magic;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNotEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;

import org.junit.jupiter.api.Test;
import org.openmbee.mdk.magic.results.ResultViews;

/** The file a query's result view is written to: the query's name, and never another query's file. */
class ResultViewFileNameTest {
  @Test
  void aPlainQualifiedNameIsTheFileName() {
    assertEquals("lander.QVTSDemo.CabinMass.result.sysml", ResultViews.fileName("lander", "QVTSDemo::CabinMass"));
  }

  @Test
  void queriesThatFlattenAlikeKeepTheirOwnFiles() {
    String nested = ResultViews.fileName("m", "A::B::Q");
    String quoted = ResultViews.fileName("m", "A::'B.Q'");
    String bound = ResultViews.fileName("m", "A::B::Q root=Pkg::car");
    assertEquals("m.A.B.Q.result.sysml", nested);
    assertNotEquals(nested, quoted);
    assertNotEquals(nested, bound);
    assertNotEquals(ResultViews.fileName("m", "A::Q x=1"), ResultViews.fileName("m", "A::Q x_1"));
    assertTrue(quoted.matches("m\\.A\\._B\\.Q_~[0-9a-f]{8}\\.result\\.sysml"), quoted);
    assertEquals(quoted, ResultViews.fileName("m", "A::'B.Q'"), "the same query writes the same file");
  }
}
