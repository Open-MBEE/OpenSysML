package org.openmbee.mdk.magic.docgen;

import java.util.ArrayList;
import java.util.List;
import org.openmbee.mdk.docgen.docbook.DBParagraph;
import org.openmbee.mdk.docgen.docbook.DBTable;
import org.openmbee.mdk.docgen.docbook.DBText;
import org.openmbee.mdk.docgen.docbook.DocumentElement;

/** Lays one run out as DocBook: a summary line, the outcome table, the rows each query answered, then diagnostics and schedule. */
public final class DocBookRenderer {
  private DocBookRenderer() {}

  public static List<DocumentElement> render(BridgeResult result) {
    List<DocumentElement> elements = new ArrayList<>();
    elements.add(new DBParagraph(summary(result)));
    if (!result.outcomes().isEmpty()) {
      DBTable table = table(result.operationLabel() + ": " + result.subject(), List.of("Outcome", "Detail", "Status"));
      for (BridgeResult.Outcome outcome : result.outcomes()) {
        table.getBody().add(row(outcome.label(), outcome.detail(), outcome.status()));
      }
      elements.add(table);
    }
    for (BridgeResult.Table answered : result.tables()) {
      elements.add(rows(answered));
    }
    if (!result.diagnostics().isEmpty()) {
      DBTable table = table("Diagnostics: " + result.subject(), List.of("Severity", "Code", "Message", "Location"));
      for (BridgeResult.Diagnostic diagnostic : result.diagnostics()) {
        table.getBody().add(row(
            diagnostic.severity(), diagnostic.code(), diagnostic.message(), diagnostic.location().orElse("")));
      }
      elements.add(table);
    }
    if (!result.schedule().isEmpty()) {
      elements.add(new DBParagraph("Schedule: " + String.join(" → ", result.schedule())));
    }
    return elements;
  }

  public static DocumentElement failure(String operationLabel, String subject, Throwable failure) {
    String message = failure.getMessage() == null ? failure.getClass().getName() : failure.getMessage();
    return new DBParagraph("[ERROR] OpenSysML " + operationLabel + " of " + subject + " failed: " + message);
  }

  /** A query's rows as a table headed by the properties it projects; an empty one still says so by its title. */
  static DBTable rows(BridgeResult.Table answered) {
    DBTable table = table(answered.query() + ": " + rowCount(answered.rows().size()), answered.columns());
    table.setShowIfEmpty(true);
    for (BridgeResult.Row row : answered.rows()) {
      List<String> cells = new ArrayList<>();
      for (int i = 0; i < answered.columns().size(); i++) {
        cells.add(i < row.cells().size() ? row.cells().get(i).text() : "");
      }
      table.getBody().add(row(cells.toArray(String[]::new)));
    }
    return table;
  }

  private static String rowCount(int rows) {
    if (rows == 0) return "no rows";
    return rows + (rows == 1 ? " row" : " rows");
  }

  static String summary(BridgeResult result) {
    StringBuilder text = new StringBuilder()
        .append("OpenSysML ").append(result.operationLabel()).append(" of ").append(result.subject())
        .append(": ").append(result.status()).append(" (").append(result.elapsedMillis()).append(" ms");
    result.finalTime().ifPresent(time -> text.append(", final time ").append(time));
    return text.append(")").toString();
  }

  private static DBTable table(String title, List<String> headers) {
    DBTable table = new DBTable();
    table.setTitle(title);
    table.setCols(headers.size());
    table.getHeaders().add(row(headers.toArray(String[]::new)));
    table.setShowIfEmpty(false);
    return table;
  }

  private static List<DocumentElement> row(String... cells) {
    List<DocumentElement> row = new ArrayList<>();
    for (String cell : cells) row.add(new DBText(cell));
    return row;
  }
}
