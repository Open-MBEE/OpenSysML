package org.openmbee.opensysml.mdk.docgen;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertInstanceOf;

import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import org.junit.jupiter.api.Test;
import org.openmbee.mdk.docgen.docbook.DBParagraph;
import org.openmbee.mdk.docgen.docbook.DBTable;
import org.openmbee.mdk.docgen.docbook.DBText;
import org.openmbee.mdk.docgen.docbook.DocumentElement;

class DocBookRendererTest {
  @Test
  void rendersSummaryOutcomeTableDiagnosticsAndSchedule() {
    List<DocumentElement> elements = DocBookRenderer.render(BridgeResult.from(Flat.verify("Model::Req", "FAILED")));
    assertEquals(4, elements.size());
    assertEquals("OpenSysML Verify of Model::Req: FAILED (42 ms)", ((DBParagraph) elements.get(0)).getText());
    DBTable outcomes = assertInstanceOf(DBTable.class, elements.get(1));
    assertEquals("Verify: Model::Req", outcomes.getTitle());
    assertEquals(3, outcomes.getCols());
    assertEquals(List.of(List.of("Outcome", "Detail", "Status")), cells(outcomes.getHeaders()));
    assertEquals(List.of(List.of("req1", "mass <= 10", "FAILED"), List.of("req2", "power <= 5", "PASSED")),
        cells(outcomes.getBody()));
    DBTable diagnostics = assertInstanceOf(DBTable.class, elements.get(2));
    assertEquals(List.of(List.of("WARNING", "choice-point", "two guards hold", "a.sysml:3:1")),
        cells(diagnostics.getBody()));
    assertEquals("Schedule: start → check → done", ((DBParagraph) elements.get(3)).getText());
  }

  @Test
  void omitsEmptySectionsAndShowsFinalTime() {
    Map<String, Object> flat = Flat.verify("S", "PASSED");
    flat.put("outcomes", List.of());
    flat.put("diagnostics", List.of());
    flat.put("schedule", List.of());
    flat.put("finalTime", "3 s");
    List<DocumentElement> elements = DocBookRenderer.render(BridgeResult.from(flat));
    assertEquals(1, elements.size());
    assertEquals("OpenSysML Verify of S: PASSED (42 ms, final time 3 s)", ((DBParagraph) elements.get(0)).getText());
  }

  @Test
  void failureBecomesAnErrorParagraph() {
    DBParagraph paragraph = (DBParagraph) DocBookRenderer.failure("Verify", "Part A", new IllegalStateException("boom"));
    assertEquals("[ERROR] OpenSysML Verify of Part A failed: boom", paragraph.getText());
  }

  private static List<List<String>> cells(List<List<DocumentElement>> rows) {
    List<List<String>> text = new ArrayList<>();
    for (List<DocumentElement> row : rows) {
      List<String> line = new ArrayList<>();
      for (DocumentElement cell : row) line.add(String.valueOf(((DBText) cell).getText()));
      text.add(line);
    }
    return text;
  }
}
