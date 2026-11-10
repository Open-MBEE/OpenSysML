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
