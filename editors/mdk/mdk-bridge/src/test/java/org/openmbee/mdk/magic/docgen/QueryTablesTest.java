package org.openmbee.mdk.magic.docgen;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertInstanceOf;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import org.junit.jupiter.api.Test;
import org.openmbee.mdk.docgen.docbook.DBParagraph;
import org.openmbee.mdk.docgen.docbook.DBTable;
import org.openmbee.mdk.docgen.docbook.DBText;
import org.openmbee.mdk.docgen.docbook.DocumentElement;

class QueryTablesTest {
  static Map<String, Object> runQuery() {
    Map<String, Object> flat = new LinkedHashMap<>();
    flat.put("operation", "RUN_QUERY");
    flat.put("operationLabel", "Run query");
    flat.put("subject", "Demo::Parts");
    flat.put("status", "PASSED");
    flat.put("elapsedMillis", 9L);
    flat.put("outcomes", List.of(Map.of("label", "Demo::Parts", "detail", "2 rows of qualifiedName, mass", "status", "PASSED", "elementId", "Demo::Parts")));
    flat.put("diagnostics", List.of());
    flat.put("schedule", List.of());
    Map<String, Object> table = new LinkedHashMap<>();
    table.put("query", "Demo::Parts");
    table.put("columns", List.of("qualifiedName", "mass"));
    table.put("rows", List.of(
        Map.of("elementId", "Demo::a", "cells", List.of(Map.of("text", "Demo::a", "elementId", "Demo::a"), Map.of("text", "1.5"))),
        Map.of("cells", List.of(Map.of("text", "Demo::b", "elementId", "Demo::b")))));
    Map<String, Object> empty = new LinkedHashMap<>();
    empty.put("query", "Demo::NoPorts");
    empty.put("columns", List.of("qualifiedName"));
    empty.put("rows", List.of());
    flat.put("tables", List.of(table, empty));
    return flat;
  }

  @Test
  void readsTablesWithTheirRowsAndCells() {
    BridgeResult result = BridgeResult.from(runQuery());
    assertEquals(2, result.tables().size());
    BridgeResult.Table table = result.tables().get(0);
    assertEquals("Demo::Parts", table.query());
    assertEquals(List.of("qualifiedName", "mass"), table.columns());
    assertEquals(new BridgeResult.Row(Optional.of("Demo::a"), List.of(
        new BridgeResult.Cell("Demo::a", Optional.of("Demo::a")), new BridgeResult.Cell("1.5", Optional.empty()))),
        table.rows().get(0));
    assertEquals(Optional.empty(), table.rows().get(1).elementId());
    assertTrue(result.tables().get(1).rows().isEmpty());
    assertTrue(BridgeResult.from(Flat.verify("S", "PASSED")).tables().isEmpty(), "runs without tables read as before");
  }

  @Test
  void rejectsATableWithoutItsQuery() {
    Map<String, Object> flat = runQuery();
    flat.put("tables", List.of(Map.of("columns", List.of("name"), "rows", List.of())));
    assertEquals("bridge result table lacks query",
        assertThrows(IllegalArgumentException.class, () -> BridgeResult.from(flat)).getMessage());
  }

  @Test
  void rendersEachQueryAsATableUnderTheSummary() {
    List<DocumentElement> elements = DocBookRenderer.render(BridgeResult.from(runQuery()));
    assertEquals(4, elements.size(), elements.toString());
    assertEquals("OpenSysML Run query of Demo::Parts: PASSED (9 ms)", ((DBParagraph) elements.get(0)).getText());
    assertEquals("Run query: Demo::Parts", assertInstanceOf(DBTable.class, elements.get(1)).getTitle());
    DBTable rows = assertInstanceOf(DBTable.class, elements.get(2));
    assertEquals("Demo::Parts: 2 rows", rows.getTitle());
    assertEquals(2, rows.getCols());
    assertEquals(List.of(List.of("qualifiedName", "mass")), cells(rows.getHeaders()));
    assertEquals(List.of(List.of("Demo::a", "1.5"), List.of("Demo::b", "")), cells(rows.getBody()),
        "a short row is padded to the columns");
    DBTable empty = assertInstanceOf(DBTable.class, elements.get(3));
    assertEquals("Demo::NoPorts: no rows", empty.getTitle());
    assertTrue(empty.isShowIfEmpty(), "an empty answer is a value, so its table still shows");
    assertTrue(empty.getBody().isEmpty());
  }

  /** A query that reported a path error crosses as its outcomes: the service's message, then each failure's own text. */
  static Map<String, Object> failedQuery() {
    Map<String, Object> flat = runQuery();
    flat.put("subject", "Demo::Handbook");
    flat.put("status", "FAILED");
    flat.put("outcomes", List.of(
        Map.of("label", "Demo::Handbook", "detail", "2 queries", "status", "FAILED", "elementId", "Demo::Handbook"),
        Map.of("label", "Demo::Parts", "detail", "2 rows of qualifiedName, mass", "status", "PASSED", "elementId", "Demo::Parts"),
        Map.of("label", "Demo::Masses", "detail", "query Demo::Masses reads feature mass of Demo::a, which has no such feature",
            "status", "FAILED", "elementId", "Demo::Masses"),
        Map.of("label", "Demo::Masses", "detail", "path error at Feature(mass): expected a feature named mass on the element;"
            + " at Demo::a; features: [mas : Real, … and 11 more]; nearest: mas", "status", "FAILED", "elementId", "Demo::a"),
        Map.of("label", "Demo::Masses", "detail", "… and 9 more path errors at Feature(mass)", "status", "FAILED")));
    flat.put("tables", List.of(((List<?>) flat.get("tables")).get(0)));
    return flat;
  }

  @Test
  void rendersAFailedQueryAsItsFailuresBesideTheRowsOfTheOthers() {
    BridgeResult result = BridgeResult.from(failedQuery());
    assertEquals(Optional.of("Demo::a"), result.outcomes().get(3).elementId());
    assertEquals(Optional.empty(), result.outcomes().get(4).elementId(), "the count of what was left out names no element");
    List<DocumentElement> elements = DocBookRenderer.render(result);
    assertEquals(3, elements.size(), elements.toString());
    assertEquals("OpenSysML Run query of Demo::Handbook: FAILED (9 ms)", ((DBParagraph) elements.get(0)).getText());
    DBTable outcomes = assertInstanceOf(DBTable.class, elements.get(1));
    assertEquals(List.of(List.of("Outcome", "Detail", "Status")), cells(outcomes.getHeaders()));
    assertEquals(List.of(
        List.of("Demo::Handbook", "2 queries", "FAILED"),
        List.of("Demo::Parts", "2 rows of qualifiedName, mass", "PASSED"),
        List.of("Demo::Masses", "query Demo::Masses reads feature mass of Demo::a, which has no such feature", "FAILED"),
        List.of("Demo::Masses", "path error at Feature(mass): expected a feature named mass on the element;"
            + " at Demo::a; features: [mas : Real, … and 11 more]; nearest: mas", "FAILED"),
        List.of("Demo::Masses", "… and 9 more path errors at Feature(mass)", "FAILED")),
        cells(outcomes.getBody()));
    assertEquals("Demo::Parts: 2 rows", assertInstanceOf(DBTable.class, elements.get(2)).getTitle(),
        "the query that answered keeps its table; the one that failed has none");
  }

  @Test
  void theRunQueryExtensionNamesItsOperation() {
    assertEquals("RUN_QUERY", new RunQuery().operation());
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
