package org.openmbee.mdk.magic.docgen;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertInstanceOf;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import org.junit.jupiter.api.Test;
import org.openmbee.mdk.docgen.docbook.DBParagraph;
import org.openmbee.mdk.docgen.docbook.DBTable;
import org.openmbee.mdk.docgen.docbook.DBText;
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
  void sysmlV2TargetsReachThePluginToo() {
    List<String> calls = new ArrayList<>();
    OpenSysMLQuery query = query((element, operation, args) -> {
      calls.add(element.getHumanName());
      return Flat.verify(element.getHumanName(), "PASSED");
    });
    query.setTargets(new ArrayList<>(List.of(new FakeV2Element("Vehicle"), new FakeElement("A"))));
    List<DocumentElement> elements = query.visit(false, null);
    assertEquals(List.of("Part Def Vehicle", "Part A"), calls);
    assertEquals("OpenSysML Verify of Part Def Vehicle: PASSED (42 ms)", ((DBParagraph) elements.get(0)).getText());
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
  void aViewIsLaidOutAsItsAttributesAndItsExposedElements() {
    BridgeResult rendered = BridgeResult.from(Flat.view(true));
    List<DocumentElement> tables = DocBookRenderer.view(rendered.views().get(0));
    DBTable header = (DBTable) tables.get(0);
    assertEquals("Query result view QResult::q", header.getTitle());
    assertEquals(4, header.getBody().size());
    assertEquals("outcome", ((DBText) header.getBody().get(2).get(0)).getText());
    assertEquals("established", ((DBText) header.getBody().get(2).get(1)).getText());
    DBTable exposed = (DBTable) tables.get(1);
    assertEquals("QResult::q: 1 row", exposed.getTitle());
    assertEquals("A::a", ((DBText) exposed.getBody().get(0).get(0)).getText());
    assertEquals(4, exposed.getHeaders().get(0).size());

    BridgeResult text = BridgeResult.from(Flat.view(false));
    assertFalse(text.views().get(0).rendered());
    List<DocumentElement> paragraphs = DocBookRenderer.view(text.views().get(0));
    assertEquals("Query result view QResult::q:", ((DBParagraph) paragraphs.get(0)).getText());
    assertTrue(String.valueOf(((DBParagraph) paragraphs.get(1)).getText()).contains("expose A::a;"));
    assertEquals(DocBookRenderer.render(BridgeResult.from(Flat.verify("x", "PASSED"))).size() + 2,
        DocBookRenderer.render(text).size(), "the view follows the run's tables");

    Map<String, Object> nameless = Flat.view(false);
    ((Map<?, ?>) ((List<?>) nameless.get("views")).get(0)).remove("name");
    assertEquals("bridge result view lacks name",
        assertThrows(IllegalArgumentException.class, () -> BridgeResult.from(nameless)).getMessage());
  }

  @Test
  void aRunnerWithoutViewsRefusesOneAndIgnoresABlank() {
    BridgeRunner runner = (element, operation, args) -> Flat.verify("x", "PASSED");
    assertEquals("PASSED", runner.run(new FakeElement("A"), "RUN_QUERY", "", "").get("status"));
    assertThrows(IllegalStateException.class, () -> runner.run(new FakeElement("A"), "RUN_QUERY", "", "text"));
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
