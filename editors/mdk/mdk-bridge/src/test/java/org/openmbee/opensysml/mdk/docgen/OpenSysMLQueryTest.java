package org.openmbee.opensysml.mdk.docgen;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertInstanceOf;
import static org.junit.jupiter.api.Assertions.assertTrue;

import java.util.ArrayList;
import java.util.List;
import org.junit.jupiter.api.Test;
import org.openmbee.mdk.docgen.docbook.DBParagraph;
import org.openmbee.mdk.docgen.docbook.DBTable;
import org.openmbee.mdk.docgen.docbook.DocumentElement;

class OpenSysMLQueryTest {
  @Test
  void runsTheOperationOnEveryElementTarget() {
    List<String> calls = new ArrayList<>();
    OpenSysMLQuery query = new OpenSysMLQuery("VERIFY", "Verify", (element, operation, args) -> {
      calls.add(operation + " " + element.getHumanName() + " [" + args + "]");
      return Flat.verify(element.getHumanName(), "PASSED");
    }) {};
    query.setTargets(new ArrayList<>(List.of(new FakeElement("A"), new FakeElement("B"))));
    List<DocumentElement> elements = query.visit(false, null);
    assertEquals(List.of("VERIFY Part A []", "VERIFY Part B []"), calls);
    assertEquals(8, elements.size());
    assertEquals("OpenSysML Verify of Part A: PASSED (42 ms)", ((DBParagraph) elements.get(0)).getText());
    assertInstanceOf(DBTable.class, elements.get(1));
    assertEquals("OpenSysML Verify of Part B: PASSED (42 ms)", ((DBParagraph) elements.get(4)).getText());
  }

  @Test
  void reportsMissingTargetsAndSkipsNonElements() {
    OpenSysMLQuery query = query((element, operation, args) -> Flat.verify("x", "PASSED"));
    assertEquals("OpenSysML Verify: no target elements.",
        ((DBParagraph) query.visit(false, null).get(0)).getText());
    query.setTargets(new ArrayList<>(List.of("a string")));
    assertEquals("OpenSysML Verify: skipped a string, not a model element.",
        ((DBParagraph) query.visit(false, null).get(0)).getText());
  }

  @Test
  void aFailingRunBecomesAnErrorParagraphAndTheOthersStillRender() {
    OpenSysMLQuery query = query((element, operation, args) -> {
      if (element.getHumanName().endsWith("A")) throw new IllegalStateException("service unavailable");
      return Flat.verify(element.getHumanName(), "PASSED");
    });
    query.setTargets(new ArrayList<>(List.of(new FakeElement("A"), new FakeElement("B"))));
    List<DocumentElement> elements = query.visit(false, null);
    assertEquals("[ERROR] OpenSysML Verify of Part A failed: service unavailable",
        ((DBParagraph) elements.get(0)).getText());
    assertEquals("OpenSysML Verify of Part B: PASSED (42 ms)", ((DBParagraph) elements.get(1)).getText());
  }

  @Test
  void ignoredQueriesRenderNothing() {
    OpenSysMLQuery query = query((element, operation, args) -> Flat.verify("x", "PASSED"));
    query.setTargets(new ArrayList<>(List.of(new FakeElement("A"))));
    query.setIgnore(true);
    assertTrue(query.visit(false, null).isEmpty());
  }

  @Test
  void concreteQueriesNameTheirOperation() {
    assertEquals("INSTANTIATE", new Instantiate().operation());
    assertEquals("EXECUTE_ACTION", new ExecuteAction().operation());
    assertEquals("EXECUTE_STATE", new ExecuteState().operation());
    assertEquals("VERIFY", new Verify().operation());
    assertEquals("EVALUATE_CALC", new EvaluateCalc().operation());
    assertEquals("RUN_ANALYSIS", new RunAnalysis().operation());
  }

  private static OpenSysMLQuery query(BridgeRunner runner) {
    return new OpenSysMLQuery("VERIFY", "Verify", runner) {};
  }

}
